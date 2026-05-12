package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type Client struct {
	BaseURL string
	Token   string
	Http    *http.Client
}

func New(baseURL string, token string) *Client {
	return &Client{
		BaseURL: baseURL,
		Token:   token,
		Http:    &http.Client{},
	}
}

func (c *Client) SetToken(token string) {
	c.Token = token
}

func (c *Client) GeneratePDF(
	projectID int,
	documentID int,
	polygonIDs []int,
	lang string,
) ([]byte, error) {

	url := fmt.Sprintf(
		"%s/api/productiondata/document/generatePdfDocument?productionDocumentId=%d",
		c.BaseURL,
		documentID,
	)

	log.Println("🌐 Request URL:", url)

	payload := map[string]interface{}{
		"productionProjectId":  projectID,
		"productionPolygonIds": polygonIDs,
		"languageCode":         lang,
	}

	body, _ := json.Marshal(payload)

	req, err := http.NewRequest(
		"POST",
		url,
		bytes.NewBuffer(body),
	)

	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/pdf")

	resp, err := c.Http.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		errorBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API ERROR (%d): %s", resp.StatusCode, string(errorBody))
	}

	pdfBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return pdfBytes, nil
}
