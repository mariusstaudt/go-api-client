package api

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

type TokenProvider func(ctx context.Context) (string, error)

func NewStaticTokenProvider(static string) TokenProvider {
	return func(_ context.Context) (string, error) {
		return static, nil
	}
}

func NewOAuth2Provider(ctx context.Context, config *clientcredentials.Config, client *http.Client) TokenProvider {
	if client == nil {
		client = &http.Client{}
	}

	oauthClient := context.WithValue(ctx, oauth2.HTTPClient, client)

	tokenSource := config.TokenSource(oauthClient)

	return func(ctx context.Context) (string, error) {
		token, err := tokenSource.Token()
		if err != nil {
			return "", err
		}
		return token.AccessToken, nil
	}
}
