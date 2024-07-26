package store

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
)

type RemoteImageSource struct {
	BaseURL string
}

type TagListResponse struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

func NewRemoteImageSource(url string) *RemoteImageSource {
	return &RemoteImageSource{BaseURL: url}
}

func (r *RemoteImageSource) SourceType() string {
	return "Remote"
}

// retrieves a list of images from  remote repository.
func (r *RemoteImageSource) List(ctx context.Context) ([]Image, error) {
	url := r.BaseURL + "/tags/list"
	authHeader, err := createAuthHeader()
	if err != nil {
		return nil, fmt.Errorf("error creating auth header: %w", err)
	}

	body, err := fetchResponseBody(url, authHeader)
	if err != nil {
		return nil, fmt.Errorf("error fetching tags list from %s: %w", url, err)
	}

	images, err := parseTagsResponse(body)
	if err != nil {
		return nil, fmt.Errorf("error parsing tags response from %s: %w", url, err)
	}

	fmt.Println("Fetched", len(images), "images:", images)
	return images, nil
}

// fetches  digest for a specific image.
func (r *RemoteImageSource) GetDigest(ctx context.Context, tag string) (string, error) {
	imageRef := fmt.Sprintf("%s:%s", r.BaseURL, tag)
	imageRef = cleanImageReference(imageRef)

	digest, err := fetchImageDigest(imageRef)
	if err != nil {
		return "", fmt.Errorf("error fetching digest for %s: %w", imageRef, err)
	}

	return digest, nil
}

// createAuthHeader generates  authorization header for HTTP requests.
func createAuthHeader() (string, error) {
	username := os.Getenv("HARBOR_USERNAME")
	password := os.Getenv("HARBOR_PASSWORD")
	if username == "" || password == "" {
		return "", fmt.Errorf("environment variables HARBOR_USERNAME or HARBOR_PASSWORD not set")
	}
	auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	return "Basic " + auth, nil
}

// fetchResponseBody makes an HTTP GET request and returns  response body.
func fetchResponseBody(url, authHeader string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for %s: %w", url, err)
	}
	req.Header.Set("Authorization", authHeader)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch response from %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body from %s: %w", url, err)
	}

	return body, nil
}

// parseTagsResponse unmarshals the tags list response and constructs image references.
func parseTagsResponse(body []byte) ([]Image, error) {
	var tagList TagListResponse
	if err := json.Unmarshal(body, &tagList); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON response: %w", err)
	}

	var images []Image
	for _, tag := range tagList.Tags {
		images = append(images, Image{Name: fmt.Sprintf("%s:%s", tagList.Name, tag)})
	}

	return images, nil
}

// cleanImageReference cleans up the image reference string.
func cleanImageReference(imageRef string) string {
	imageRef = imageRef[strings.Index(imageRef, "//")+2:]
	return strings.ReplaceAll(imageRef, "/v2", "")
}

// fetchImageDigest retrieves  digest for an image reference.
func fetchImageDigest(imageRef string) (string, error) {
	username := os.Getenv("HARBOR_USERNAME")
	password := os.Getenv("HARBOR_PASSWORD")

	digest, err := crane.Digest(imageRef, crane.WithAuth(&authn.Basic{
		Username: username,
		Password: password,
	}), crane.Insecure)
	if err != nil {
		return "", fmt.Errorf("failed to fetch digest for %s: %w", imageRef, err)
	}

	return digest, nil
}
