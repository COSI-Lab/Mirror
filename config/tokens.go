package config

import (
	"io"

	"github.com/pelletier/go-toml/v2"
)

// Tokens is what we unmarshal the tokens.toml file into
type Tokens struct {
	Tokens []Token `toml:"tokens"`
}

func ReadTokens(r io.Reader) (tokens *Tokens, err error) {
	err = toml.NewDecoder(r).Decode(&tokens)
	if err != nil {
		return nil, err
	}
	return tokens, nil
}

// GetToken returns the token struct by token string
func (tokens *Tokens) GetToken(token string) *Token {
	for _, t := range tokens.Tokens {
		if t.Token == token {
			return &t
		}
	}
	return nil
}

// Token is the struct that represents a single access token in the tokens.txt file
//
// The token is able to trigger manual syncs for particular projects, if the project list is empty then all projects are allowed
type Token struct {
	Name     string   `toml:"name"`
	Token    string   `toml:"token"`
	Projects []string `toml:"projects"`
}

func (token *Token) HasProject(project string) bool {
	// Empty project list means all projects
	if len(token.Projects) == 0 {
		return true
	}

	for _, p := range token.Projects {
		if p == project {
			return true
		}
	}

	return false
}
