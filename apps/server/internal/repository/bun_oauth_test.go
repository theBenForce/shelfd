package repository_test

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/shelfd/shelfd/internal/repository"
)

func TestBunOAuthModelTableNames(t *testing.T) {
	// Initialize Bun DB with PostgreSQL dialect using a mock / dummy connector
	connector := pgdriver.NewConnector(pgdriver.WithDSN("postgres://mock:mock@localhost:5432/mock?sslmode=disable"))
	sqldb := sql.OpenDB(connector)
	defer sqldb.Close()

	db := bun.NewDB(sqldb, pgdialect.New())
	defer db.Close()

	// 1. Verify OAuthClient INSERT query string targets oauth_clients
	client := &repository.OAuthClient{
		ID:            "client_test_123",
		ClientName:    "Test Client",
		RedirectURIs:  "https://example.com/callback",
		GrantTypes:    "authorization_code",
		ResponseTypes: "code",
		CreatedAt:     time.Now().UTC(),
	}

	insertClientSQL := db.NewInsert().Model(client).String()
	if strings.Contains(insertClientSQL, "o_auth_clients") {
		t.Fatalf("OAuthClient insert query mistakenly targeted 'o_auth_clients': %s", insertClientSQL)
	}
	if !strings.Contains(insertClientSQL, "oauth_clients") {
		t.Fatalf("OAuthClient insert query did not target 'oauth_clients': %s", insertClientSQL)
	}

	// 2. Verify OAuthClient SELECT query string targets oauth_clients
	selectClientSQL := db.NewSelect().Model(client).Where("id = ?", "client_test_123").String()
	if strings.Contains(selectClientSQL, "o_auth_clients") {
		t.Fatalf("OAuthClient select query mistakenly targeted 'o_auth_clients': %s", selectClientSQL)
	}
	if !strings.Contains(selectClientSQL, "oauth_clients") {
		t.Fatalf("OAuthClient select query did not target 'oauth_clients': %s", selectClientSQL)
	}

	// 3. Verify OAuthCode INSERT query string targets oauth_codes
	code := &repository.OAuthCode{
		Code:        "code_test_123",
		ClientID:    "client_test_123",
		UserID:      "user_test_123",
		RedirectURI: "https://example.com/callback",
		ExpiresAt:   time.Now().UTC().Add(10 * time.Minute),
		CreatedAt:   time.Now().UTC(),
	}

	insertCodeSQL := db.NewInsert().Model(code).String()
	if strings.Contains(insertCodeSQL, "o_auth_codes") {
		t.Fatalf("OAuthCode insert query mistakenly targeted 'o_auth_codes': %s", insertCodeSQL)
	}
	if !strings.Contains(insertCodeSQL, "oauth_codes") {
		t.Fatalf("OAuthCode insert query did not target 'oauth_codes': %s", insertCodeSQL)
	}

	// 4. Verify OAuthCode SELECT query string targets oauth_codes
	selectCodeSQL := db.NewSelect().Model(code).Where("code = ?", "code_test_123").String()
	if strings.Contains(selectCodeSQL, "o_auth_codes") {
		t.Fatalf("OAuthCode select query mistakenly targeted 'o_auth_codes': %s", selectCodeSQL)
	}
	if !strings.Contains(selectCodeSQL, "oauth_codes") {
		t.Fatalf("OAuthCode select query did not target 'oauth_codes': %s", selectCodeSQL)
	}
}
