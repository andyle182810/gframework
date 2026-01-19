//nolint:paralleltest,varnamelen
package repo_test

import (
	"context"
	"testing"
	"time"

	"github.com/andyle182810/gframework/examples/demo-api/internal/repo"
	"github.com/andyle182810/gframework/postgres"
	"github.com/andyle182810/gframework/testutil"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/tracelog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRepo(ctx context.Context, t *testing.T) (*repo.UserRepo, *postgres.Postgres) { //nolint:unparam
	t.Helper()

	pgContainer := testutil.SetupPostgresContainer(ctx, t)

	pg, err := postgres.New(&postgres.Config{ //nolint:contextcheck
		URL:                      pgContainer.ConnectionString(),
		MaxConnection:            10,
		MinConnection:            2,
		MaxConnectionIdleTime:    5 * time.Minute,
		MaxConnectionLifetime:    30 * time.Minute,
		HealthCheckPeriod:        1 * time.Minute,
		ConnectTimeout:           5 * time.Second,
		LogLevel:                 tracelog.LogLevelInfo,
		StatementTimeout:         30 * time.Second,
		LockTimeout:              10 * time.Second,
		IdleInTransactionTimeout: 30 * time.Second,
	})
	require.NoError(t, err)

	err = testutil.RunMigrations(ctx, pg, "../../db/migrations")
	require.NoError(t, err)

	testutil.CleanupDatabase(t, ctx, pg)

	repo := repo.NewUserRepo(pg.DBPool)

	return repo, pg
}

func TestUserRepo_CreateUser(t *testing.T) {
	testutil.SkipIfShort(t)

	ctx := testutil.Context(t)
	repo, _ := setupTestRepo(ctx, t)

	tests := []struct {
		name      string
		userName  string
		userEmail string
		wantErr   bool
	}{
		{
			name:      "successful user creation",
			userName:  "John Doe",
			userEmail: testutil.RandomEmail(),
			wantErr:   false,
		},
		{
			name:      "create another user",
			userName:  "Jane Smith",
			userEmail: testutil.RandomEmail(),
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := repo.CreateUser(ctx, tt.userName, tt.userEmail)

			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, user)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, user)
				assert.NotEmpty(t, user.ID)
				assert.Equal(t, tt.userName, user.Name)
				assert.Equal(t, tt.userEmail, user.Email)
				assert.NotZero(t, user.CreatedAt)
				assert.NotZero(t, user.UpdatedAt)

				_, err := uuid.Parse(user.ID)
				assert.NoError(t, err)
			}
		})
	}
}

func TestUserRepo_CreateUser_DuplicateEmail(t *testing.T) {
	testutil.SkipIfShort(t)

	ctx := testutil.Context(t)
	repo, _ := setupTestRepo(ctx, t)

	email := testutil.RandomEmail()

	user1, err := repo.CreateUser(ctx, "First User", email)
	require.NoError(t, err)
	require.NotNil(t, user1)

	user2, err := repo.CreateUser(ctx, "Second User", email)
	require.Error(t, err)
	require.Nil(t, user2)
}

func TestUserRepo_GetUserByID(t *testing.T) {
	testutil.SkipIfShort(t)

	ctx := testutil.Context(t)
	repo, _ := setupTestRepo(ctx, t)

	createdUser, err := repo.CreateUser(ctx, "Test User", testutil.RandomEmail())
	require.NoError(t, err)

	tests := []struct {
		name    string
		userID  string
		wantErr bool
	}{
		{
			name:    "get existing user",
			userID:  createdUser.ID,
			wantErr: false,
		},
		{
			name:    "get non-existent user",
			userID:  uuid.NewString(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := repo.GetUserByID(ctx, tt.userID)

			if tt.wantErr {
				require.Error(t, err)
				require.True(t, pgxscan.NotFound(err))
			} else {
				require.NoError(t, err)
				assert.NotNil(t, user)
				assert.Equal(t, tt.userID, user.ID)
				assert.Equal(t, createdUser.Name, user.Name)
				assert.Equal(t, createdUser.Email, user.Email)
			}
		})
	}
}

func TestUserRepo_GetUserByEmail(t *testing.T) {
	testutil.SkipIfShort(t)

	ctx := testutil.Context(t)
	repo, _ := setupTestRepo(ctx, t)

	email := testutil.RandomEmail()
	createdUser, err := repo.CreateUser(ctx, "Test User", email)
	require.NoError(t, err)

	tests := []struct {
		name      string
		userEmail string
		wantErr   bool
	}{
		{
			name:      "get existing user by email",
			userEmail: email,
			wantErr:   false,
		},
		{
			name:      "get non-existent user by email",
			userEmail: testutil.RandomEmail(),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := repo.GetUserByEmail(ctx, tt.userEmail)

			if tt.wantErr {
				require.Error(t, err)
				require.True(t, pgxscan.NotFound(err))
			} else {
				require.NoError(t, err)
				assert.NotNil(t, user)
				assert.Equal(t, createdUser.ID, user.ID)
				assert.Equal(t, createdUser.Name, user.Name)
				assert.Equal(t, tt.userEmail, user.Email)
			}
		})
	}
}

func TestUserRepo_ListUsers(t *testing.T) {
	testutil.SkipIfShort(t)

	ctx := testutil.Context(t)
	repo, _ := setupTestRepo(ctx, t)

	const userCount = 25
	for range userCount {
		_, err := repo.CreateUser(ctx, testutil.RandomString(10), testutil.RandomEmail())
		require.NoError(t, err)
	}

	tests := []struct {
		name          string
		limit         int
		offset        int
		expectedCount int
	}{
		{
			name:          "list first 10 users",
			limit:         10,
			offset:        0,
			expectedCount: 10,
		},
		{
			name:          "list next 10 users",
			limit:         10,
			offset:        10,
			expectedCount: 10,
		},
		{
			name:          "list last users",
			limit:         10,
			offset:        20,
			expectedCount: 5,
		},
		{
			name:          "list all users",
			limit:         100,
			offset:        0,
			expectedCount: userCount,
		},
		{
			name:          "empty result",
			limit:         10,
			offset:        100,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users, err := repo.ListUsers(ctx, tt.limit, tt.offset)
			require.NoError(t, err)
			assert.Len(t, users, tt.expectedCount)

			if len(users) > 1 {
				for i := range len(users) - 1 {
					assert.True(t, users[i].CreatedAt.After(users[i+1].CreatedAt) ||
						users[i].CreatedAt.Equal(users[i+1].CreatedAt),
						"Users should be ordered by created_at DESC")
				}
			}
		})
	}
}

func TestUserRepo_CountUsers(t *testing.T) {
	testutil.SkipIfShort(t)

	ctx := testutil.Context(t)
	repo, _ := setupTestRepo(ctx, t)

	count, err := repo.CountUsers(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	const userCount = 15
	for range userCount {
		_, err := repo.CreateUser(ctx, testutil.RandomString(10), testutil.RandomEmail())
		require.NoError(t, err)
	}

	count, err = repo.CountUsers(ctx)
	require.NoError(t, err)
	assert.Equal(t, userCount, count)
}

func TestUserRepo_UpdateUser(t *testing.T) {
	testutil.SkipIfShort(t)

	ctx := testutil.Context(t)
	repo, _ := setupTestRepo(ctx, t)

	originalEmail := testutil.RandomEmail()
	createdUser, err := repo.CreateUser(ctx, "Original Name", originalEmail)
	require.NoError(t, err)

	newName := "Updated Name"
	newEmail := testutil.RandomEmail()

	updatedUser, err := repo.UpdateUser(ctx, createdUser.ID, newName, newEmail)
	require.NoError(t, err)
	assert.NotNil(t, updatedUser)
	assert.Equal(t, createdUser.ID, updatedUser.ID)
	assert.Equal(t, newName, updatedUser.Name)
	assert.Equal(t, newEmail, updatedUser.Email)
	assert.Equal(t, createdUser.CreatedAt.Unix(), updatedUser.CreatedAt.Unix())
	assert.True(t, updatedUser.UpdatedAt.After(updatedUser.CreatedAt) ||
		updatedUser.UpdatedAt.Equal(updatedUser.CreatedAt))

	fetchedUser, err := repo.GetUserByID(ctx, createdUser.ID)
	require.NoError(t, err)
	assert.Equal(t, newName, fetchedUser.Name)
	assert.Equal(t, newEmail, fetchedUser.Email)
}

func TestUserRepo_UpdateUser_NonExistent(t *testing.T) {
	testutil.SkipIfShort(t)

	ctx := testutil.Context(t)
	repo, _ := setupTestRepo(ctx, t)

	updatedUser, err := repo.UpdateUser(ctx, uuid.NewString(), "Name", testutil.RandomEmail())
	require.Error(t, err)
	require.Nil(t, updatedUser)
	assert.True(t, pgxscan.NotFound(err))
}

func TestUserRepo_DeleteUser(t *testing.T) {
	testutil.SkipIfShort(t)

	ctx := testutil.Context(t)
	repo, _ := setupTestRepo(ctx, t)

	createdUser, err := repo.CreateUser(ctx, "Test User", testutil.RandomEmail())
	require.NoError(t, err)

	err = repo.DeleteUser(ctx, createdUser.ID)
	require.NoError(t, err)

	fetchedUser, err := repo.GetUserByID(ctx, createdUser.ID)
	require.Error(t, err)
	require.Nil(t, fetchedUser)
	require.True(t, pgxscan.NotFound(err))
}

func TestUserRepo_DeleteUser_NonExistent(t *testing.T) {
	testutil.SkipIfShort(t)

	ctx := testutil.Context(t)
	repo, _ := setupTestRepo(ctx, t)

	err := repo.DeleteUser(ctx, uuid.NewString())
	assert.NoError(t, err)
}

func TestUserRepo_ConcurrentCreation(t *testing.T) {
	testutil.SkipIfShort(t)

	ctx := testutil.Context(t)
	repo, _ := setupTestRepo(ctx, t)

	const concurrentUsers = 10
	errChan := make(chan error, concurrentUsers)

	for range concurrentUsers {
		go func() {
			_, err := repo.CreateUser(ctx, testutil.RandomString(10), testutil.RandomEmail())
			errChan <- err
		}()
	}

	for range concurrentUsers {
		err := <-errChan
		require.NoError(t, err)
	}

	count, err := repo.CountUsers(ctx)
	require.NoError(t, err)
	assert.Equal(t, concurrentUsers, count)
}
