package testutil

import (
	"context"
	"fmt"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestContainers holds all test containers
type TestContainers struct {
	PostgresContainer testcontainers.Container
	RedisContainer    testcontainers.Container
	PostgresURL       string
	RedisURL          string
}

// StartContainers starts all required containers for testing
func StartContainers(ctx context.Context) (*TestContainers, error) {
	tc := &TestContainers{}

	// Start PostgreSQL
	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:15-alpine"),
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start postgres container: %w", err)
	}
	tc.PostgresContainer = pgContainer

	pgHost, err := pgContainer.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get postgres host: %w", err)
	}

	pgPort, err := pgContainer.MappedPort(ctx, "5432")
	if err != nil {
		return nil, fmt.Errorf("failed to get postgres port: %w", err)
	}

	tc.PostgresURL = fmt.Sprintf("postgres://testuser:testpass@%s:%s/testdb?sslmode=disable", pgHost, pgPort.Port())

	// Start Redis
	redisContainer, err := redis.RunContainer(ctx,
		testcontainers.WithImage("redis:7-alpine"),
		redis.WithLogLevel(redis.LogLevelDebug),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start redis container: %w", err)
	}
	tc.RedisContainer = redisContainer

	redisHost, err := redisContainer.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get redis host: %w", err)
	}

	redisPort, err := redisContainer.MappedPort(ctx, "6379")
	if err != nil {
		return nil, fmt.Errorf("failed to get redis port: %w", err)
	}

	tc.RedisURL = fmt.Sprintf("redis://%s:%s", redisHost, redisPort.Port())

	return tc, nil
}

// Cleanup terminates all containers
func (tc *TestContainers) Cleanup(ctx context.Context) error {
	var errs []error

	if tc.PostgresContainer != nil {
		if err := tc.PostgresContainer.Terminate(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to terminate postgres: %w", err))
		}
	}

	if tc.RedisContainer != nil {
		if err := tc.RedisContainer.Terminate(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to terminate redis: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}

	return nil
}

// WaitForHealthy waits for all services to be healthy
func (tc *TestContainers) WaitForHealthy(ctx context.Context) error {
	// Additional health checks can be added here if needed
	// The containers already have wait strategies, but we can add more checks
	time.Sleep(2 * time.Second) // Give services a moment to fully initialize
	return nil
}
