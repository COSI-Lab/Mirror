package config_test

import (
	"strings"
	"testing"

	"github.com/COSI-Lab/Mirror/config"
)

// Test tokens.toml parsing
func TestTokens(t *testing.T) {
	example := `
		[[tokens]]
		name = "Example"
		token = "1234"
		projects = ["archlinux", "archlinux32"]

		[[tokens]]
		name = "All"
		token = "5678"
		projects = []
	`

	// Reader from example string
	reader := strings.NewReader(example)
	tokens, err := config.ReadTokens(reader)
	if err != nil {
		t.Errorf("Expected no error, got %s", err)
	}

	// Check that we have 2 tokens
	if len(tokens.Tokens) != 2 {
		t.Errorf("Expected 2 tokens, got %d", len(tokens.Tokens))
	}

	// Check that the first token is correct
	if tokens.Tokens[0].Name != "Example" {
		t.Errorf("Expected token name Example, got %s", tokens.Tokens[0].Name)
	}

	if tokens.Tokens[0].Token != "1234" {
		t.Errorf("Expected token 1234, got %s", tokens.Tokens[0].Token)
	}

	if len(tokens.Tokens[0].Projects) != 2 {
		t.Errorf("Expected 2 projects, got %d", len(tokens.Tokens[0].Projects))
	}

	for _, project := range tokens.Tokens[0].Projects {
		if project != "archlinux" && project != "archlinux32" {
			t.Errorf("Expected project archlinux or archlinux32, got %s", project)
		}
	}

	// Check that the second token is correct
	if tokens.Tokens[1].Name != "All" {
		t.Errorf("Expected token name All, got %s", tokens.Tokens[1].Name)
	}

	if tokens.Tokens[1].Token != "5678" {
		t.Errorf("Expected token 5678, got %s", tokens.Tokens[1].Token)
	}

	if len(tokens.Tokens[1].Projects) != 0 {
		t.Errorf("Expected 0 projects, got %d", len(tokens.Tokens[1].Projects))
	}

	// Check that GetToken works
	if tokens.GetToken("0000") != nil {
		t.Errorf("Expected nil token, got %s", tokens.GetToken("0000").Token)
	}

	// Check that HasProject works
	if !tokens.GetToken("1234").HasProject("archlinux") {
		t.Errorf("Expected token to have project archlinux")
	}

	if !tokens.GetToken("1234").HasProject("archlinux32") {
		t.Errorf("Expected token to have project archlinux32")
	}

	if tokens.GetToken("1234").HasProject("blender") {
		t.Errorf("Expected token to not have project blender")
	}

	if !tokens.GetToken("5678").HasProject("archlinux") {
		t.Errorf("Expected token to have project archlinux")
	}
}
