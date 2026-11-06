package pg

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	storepg "github.com/chaitin/panda-wiki/store/pg"
)

// These tests exercise sequential inserts against PostgreSQL, including the
// existing count-then-insert behavior. They do not assert concurrent admission.
func TestQuotaIntegrationNodes(t *testing.T) {
	db := newQuotaIntegrationDB(t)
	repository := NewNodeRepository(db, log.NewLogger(&config.Config{}))
	kbID := uuid.NewString()

	// One folder and 299 documents consume the default quota of 300 nodes.
	seed := make([]domain.Node, 300)
	for i := range seed {
		seed[i] = domain.Node{
			ID:       uuid.NewString(),
			KBID:     kbID,
			Name:     fmt.Sprintf("seed-%d", i),
			Type:     domain.NodeTypeDocument,
			Position: float64(i + 1),
		}
	}
	seed[0].Type = domain.NodeTypeFolder
	require.NoError(t, db.CreateInBatches(&seed, 100).Error)

	ctx := context.Background()
	request := &domain.CreateNodeReq{
		KBID:    kbID,
		NavId:   "quota-test-nav",
		Name:    "boundary-document",
		Type:    domain.NodeTypeDocument,
		Content: "quota integration test",
		MaxNode: domain.GetBaseEditionLimitation(ctx).MaxNode,
	}

	id, err := repository.Create(ctx, request, "quota-test-author")
	require.ErrorIs(t, err, domain.ErrMaxNodeLimitReached)
	require.Empty(t, id)

	configuredCtx := domain.WithBaseEditionQuotas(ctx, 301, 2)
	request.MaxNode = domain.GetBaseEditionLimitation(configuredCtx).MaxNode
	id, err = repository.Create(configuredCtx, request, "quota-test-author")
	require.NoError(t, err, "the 301st node should fit the configured quota")
	require.NotEmpty(t, id)
	createdID := id

	id, err = repository.Create(configuredCtx, request, "quota-test-author")
	require.ErrorIs(t, err, domain.ErrMaxNodeLimitReached, "the 302nd node should be rejected")
	require.Empty(t, id)

	reducedCtx := domain.WithBaseEditionQuotas(ctx, 300, 1)
	request.MaxNode = domain.GetBaseEditionLimitation(reducedCtx).MaxNode
	_, err = repository.Create(reducedCtx, request, "quota-test-author")
	require.ErrorIs(t, err, domain.ErrMaxNodeLimitReached)

	var count int64
	require.NoError(t, db.Model(&domain.Node{}).Where("kb_id = ?", kbID).Count(&count).Error)
	require.EqualValues(t, 301, count, "lowering the quota must preserve existing nodes")
	var created domain.Node
	require.NoError(t, db.First(&created, "id = ?", createdID).Error)
	require.Equal(t, request.Content, created.Content)
}

func TestQuotaIntegrationBackendAccounts(t *testing.T) {
	db := newQuotaIntegrationDB(t)
	repository := NewUserRepository(db, log.NewLogger(&config.Config{}))
	ctx := context.Background()
	const password = "QuotaIntegrationPassword!"
	newUser := func(account string, role consts.UserRole) *domain.User {
		return &domain.User{
			ID:       uuid.NewString(),
			Account:  account,
			Password: password,
			Role:     role,
		}
	}

	require.NoError(t, repository.CreateUser(ctx, newUser("admin", consts.UserRoleAdmin), consts.LicenseEditionFree))
	err := repository.CreateUser(ctx, newUser("editor", consts.UserRoleUser), consts.LicenseEditionFree)
	require.ErrorIs(t, err, domain.ErrMaxAdminLimitReached, "the default quota should reject the second account")

	configuredCtx := domain.WithBaseEditionQuotas(ctx, 301, 2)
	err = repository.CreateUser(configuredCtx, newUser("editor", consts.UserRoleUser), consts.LicenseEditionFree)
	require.NoError(t, err, "the second account should fit the configured quota")
	user, err := repository.VerifyUser(configuredCtx, "editor", password)
	require.NoError(t, err)
	require.Equal(t, consts.UserRoleUser, user.Role)

	err = repository.CreateUser(configuredCtx, newUser("third", consts.UserRoleAdmin), consts.LicenseEditionFree)
	require.ErrorIs(t, err, domain.ErrMaxAdminLimitReached, "both admin and ordinary backend accounts count toward the quota")

	reducedCtx := domain.WithBaseEditionQuotas(ctx, 300, 1)
	err = repository.CreateUser(reducedCtx, newUser("third", consts.UserRoleUser), consts.LicenseEditionFree)
	require.ErrorIs(t, err, domain.ErrMaxAdminLimitReached)

	var accounts []string
	require.NoError(t, db.Model(&domain.User{}).Order("account").Pluck("account", &accounts).Error)
	require.Equal(t, []string{"admin", "editor"}, accounts, "lowering the quota must preserve existing accounts")
}

// Each test pins one PostgreSQL connection to its own schema. AutoMigrate and
// cleanup operate only in that schema; existing application tables are untouched.
func newQuotaIntegrationDB(t *testing.T) *storepg.DB {
	t.Helper()
	dsn := os.Getenv("PANDAWIKI_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set PANDAWIKI_TEST_POSTGRES_DSN to run PostgreSQL quota integration tests")
	}

	sqlDB, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, sqlDB.Close())
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	conn, err := sqlDB.Conn(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
	})

	schema := pq.QuoteIdentifier("panda_quota_" + uuid.NewString())
	_, err = conn.ExecContext(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanupCancel()
		_, cleanupErr := conn.ExecContext(cleanupCtx, "DROP SCHEMA "+schema+" CASCADE")
		require.NoError(t, cleanupErr)
	})
	_, err = conn.ExecContext(ctx, "SET search_path TO "+schema)
	require.NoError(t, err)

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: conn}), &gorm.Config{
		DisableAutomaticPing: true,
		Logger:               logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&domain.Node{}, &domain.User{}))
	return &storepg.DB{DB: db}
}
