package ghl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	BaseURL     = "https://services.leadconnectorhq.com"
	AuthURL     = "https://marketplace.gohighlevel.com"
	APIVersion  = "2021-07-28"
)

type Client struct {
	httpClient   *http.Client
	clientID     string
	clientSecret string
}

func NewClient(clientID, clientSecret string) *Client {
	return &Client{
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		clientID:     clientID,
		clientSecret: clientSecret,
	}
}

// ============================================================
// OAuth
// ============================================================

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	LocationID   string `json:"locationId"`
	CompanyID    string `json:"companyId"`
	UserType     string `json:"userType"`
}

func (c *Client) ExchangeCode(ctx context.Context, code, redirectURI string) (*TokenResponse, error) {
	data := url.Values{
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
	}
	return c.exchangeToken(ctx, data)
}

func (c *Client) RefreshAccessToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	data := url.Values{
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}
	return c.exchangeToken(ctx, data)
}

func (c *Client) exchangeToken(ctx context.Context, data url.Values) (*TokenResponse, error) {
	// GHL token endpoint is on the services domain, not the marketplace domain
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		BaseURL+"/oauth/token",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GHL token exchange failed %d: %s", resp.StatusCode, string(body))
	}

	var tr TokenResponse
	return &tr, json.NewDecoder(resp.Body).Decode(&tr)
}

// ============================================================
// Contacts
// ============================================================

type Contact struct {
	ID          string   `json:"id"`
	FirstName   string   `json:"firstName"`
	LastName    string   `json:"lastName"`
	Email       string   `json:"email"`
	Phone       string   `json:"phone"`
	LocationID  string   `json:"locationId"`
	Tags        []string `json:"tags"`
}

type CreateContactRequest struct {
	FirstName  string   `json:"firstName,omitempty"`
	LastName   string   `json:"lastName,omitempty"`
	Phone      string   `json:"phone,omitempty"`
	Email      string   `json:"email,omitempty"`
	LocationID string   `json:"locationId"`
	Tags       []string `json:"tags,omitempty"`
	Source     string   `json:"source,omitempty"`
}

func (c *Client) CreateContact(ctx context.Context, accessToken string, req *CreateContactRequest) (*Contact, error) {
	var result struct {
		Contact Contact `json:"contact"`
	}
	err := c.post(ctx, accessToken, "/contacts/", req, &result)
	if err != nil {
		return nil, err
	}
	return &result.Contact, nil
}

func (c *Client) GetContact(ctx context.Context, accessToken, contactID string) (*Contact, error) {
	var result struct {
		Contact Contact `json:"contact"`
	}
	err := c.get(ctx, accessToken, "/contacts/"+contactID, &result)
	return &result.Contact, err
}

// ============================================================
// Conversations
// ============================================================

type Conversation struct {
	ID         string `json:"id"`
	ContactID  string `json:"contactId"`
	LocationID string `json:"locationId"`
	Type       string `json:"type"`
}

type CreateConversationRequest struct {
	ContactID  string `json:"contactId"`
	LocationID string `json:"locationId"`
}

func (c *Client) CreateConversation(ctx context.Context, accessToken string, req *CreateConversationRequest) (*Conversation, error) {
	var result struct {
		Conversation Conversation `json:"conversation"`
	}
	err := c.post(ctx, accessToken, "/conversations/", req, &result)
	return &result.Conversation, err
}

func (c *Client) GetOrCreateConversation(ctx context.Context, accessToken, locationID, contactID string) (*Conversation, error) {
	// Search for existing conversation
	var searchResult struct {
		Conversations []Conversation `json:"conversations"`
	}
	err := c.get(ctx, accessToken,
		fmt.Sprintf("/conversations/search?locationId=%s&contactId=%s", locationID, contactID),
		&searchResult,
	)
	if err == nil && len(searchResult.Conversations) > 0 {
		return &searchResult.Conversations[0], nil
	}

	return c.CreateConversation(ctx, accessToken, &CreateConversationRequest{
		ContactID:  contactID,
		LocationID: locationID,
	})
}

// ============================================================
// Messages
// ============================================================

type SendMessageRequest struct {
	Type                   string   `json:"type"`
	ContactID              string   `json:"contactId"`
	ConversationProviderId string   `json:"conversationProviderId,omitempty"`
	Message                string   `json:"message"`
	Attachments            []string `json:"attachments,omitempty"`
}

type SendMessageResponse struct {
	MessageID string `json:"messageId"`
}

func (c *Client) SendConversationMessage(ctx context.Context, accessToken string, req *SendMessageRequest) (*SendMessageResponse, error) {
	var result SendMessageResponse
	err := c.post(ctx, accessToken, "/conversations/messages", req, &result)
	return &result, err
}

// LogInboundMessage logs an inbound message (received from a contact) into GHL
// via the custom conversation provider inbound endpoint.
func (c *Client) LogInboundMessage(ctx context.Context, accessToken string, req *SendMessageRequest) (*SendMessageResponse, error) {
	var result SendMessageResponse
	err := c.post(ctx, accessToken, "/conversations/messages/inbound", req, &result)
	return &result, err
}

// ============================================================
// HTTP helpers
// ============================================================

func (c *Client) post(ctx context.Context, accessToken, path string, body, result interface{}) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, BaseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	c.setHeaders(req, accessToken)
	return c.do(req, result)
}

func (c *Client) get(ctx context.Context, accessToken, path string, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, BaseURL+path, nil)
	if err != nil {
		return err
	}
	c.setHeaders(req, accessToken)
	return c.do(req, result)
}

func (c *Client) setHeaders(req *http.Request, accessToken string) {
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Version", APIVersion)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
}

func (c *Client) do(req *http.Request, result interface{}) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("GHL request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GHL API error %d: %s", resp.StatusCode, string(body))
	}

	if result == nil {
		return nil
	}
	return json.Unmarshal(body, result)
}
