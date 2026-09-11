package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
)

type websiteCrawlPage struct {
	URL   string `json:"url"`
	Title string `json:"title"`
	Text  string `json:"text"`
}
type websiteCrawlResponse struct {
	Pages      []websiteCrawlPage `json:"pages"`
	Truncated  bool               `json:"truncated"`
	Discovered int                `json:"discovered"`
}

func validateWebsiteURL(raw string) (string, string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return "", "", fmt.Errorf("invalid website URL")
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" {
		return "", "", fmt.Errorf("local website URL is not allowed")
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast()) {
		return "", "", fmt.Errorf("private website URL is not allowed")
	}
	parsed.Fragment = ""
	parsed.RawQuery = ""
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	return parsed.String(), strings.TrimPrefix(host, "www."), nil
}

func crawlWebsite(ctx context.Context, source websiteIngestionSource) (string, error) {
	endpoint, err := documentParserEndpoint()
	if err != nil {
		return "", err
	}
	endpoint = strings.TrimSuffix(endpoint, "/parse-document") + "/crawl-website"
	payload, _ := json.Marshal(map[string]any{"url": source.URL, "max_pages": source.MaxPages, "max_depth": source.MaxDepth})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := KnowledgeIngestionService.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("website crawler request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		var failure struct {
			Detail string `json:"detail"`
		}
		_ = json.NewDecoder(response.Body).Decode(&failure)
		if failure.Detail == "" {
			failure.Detail = response.Status
		}
		return "", fmt.Errorf("website crawler failed: %s", failure.Detail)
	}
	var crawled websiteCrawlResponse
	if err := json.NewDecoder(response.Body).Decode(&crawled); err != nil {
		return "", err
	}
	if len(crawled.Pages) == 0 {
		return "", fmt.Errorf("website contains no readable pages")
	}
	var output strings.Builder
	fmt.Fprintf(&output, "# 官网内容\n\n抓取入口：%s\n\n", source.URL)
	for _, page := range crawled.Pages {
		title := strings.TrimSpace(page.Title)
		if title == "" {
			title = page.URL
		}
		fmt.Fprintf(&output, "# %s\n\n来源：%s\n\n%s\n\n", title, page.URL, strings.TrimSpace(page.Text))
	}
	return strings.TrimSpace(output.String()), nil
}
