package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	authnv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// setupTestGVR initializes the GetAgenticSessionV1Alpha1Resource function for tests
func setupTestGVR() {
	GetAgenticSessionV1Alpha1Resource = func() schema.GroupVersionResource {
		return schema.GroupVersionResource{
			Group:    "vteam.ambient-code",
			Version:  "v1alpha1",
			Resource: "agenticsessions",
		}
	}
}

func TestMintSessionVertexCredentials_Success(t *testing.T) {
	// Setup backend namespace (backend reads secret from its own namespace)
	Namespace = "backend-namespace"

	// Setup fake K8s clients
	vertexSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ambient-vertex",
			Namespace: "backend-namespace", // Must match Namespace variable
		},
		Data: map[string][]byte{
			"ambient-code-key.json": []byte(`{"type":"service_account","project_id":"test"}`),
			"project-id":            []byte("test-project-123"),
			"region":                []byte("us-central1"),
		},
	}

	fakeClient := k8sfake.NewSimpleClientset(vertexSecret)
	K8sClient = fakeClient

	// Create AgenticSession CR
	sessionCR := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "vteam.ambient-code/v1alpha1",
			"kind":       "AgenticSession",
			"metadata": map[string]interface{}{
				"name":      "test-session",
				"namespace": "test-project",
				"annotations": map[string]interface{}{
					"ambient-code.io/runner-sa": "test-session-runner",
				},
			},
		},
	}

	s := runtime.NewScheme()
	DynamicClient = fake.NewSimpleDynamicClient(s, sessionCR)
	setupTestGVR()
	setupTestGVR()

	// Setup Gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/projects/:projectName/agentic-sessions/:sessionName/vertex/credentials", MintSessionVertexCredentials)

	// Create request with valid BOT_TOKEN
	req := httptest.NewRequest("POST", "/api/projects/test-project/agentic-sessions/test-session/vertex/credentials", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	// Mock TokenReview to return valid ServiceAccount
	fakeClient.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, &authnv1.TokenReview{
			Status: authnv1.TokenReviewStatus{
				Authenticated: true,
				User: authnv1.UserInfo{
					Username: "system:serviceaccount:test-project:test-session-runner",
				},
			},
		}, nil
	})

	// Execute request
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify response contains credentials
	if _, ok := response["credentials"]; !ok {
		t.Error("Response missing 'credentials' field")
	}
	if response["projectId"] != "test-project-123" {
		t.Errorf("Expected projectId 'test-project-123', got %v", response["projectId"])
	}
	if response["region"] != "us-central1" {
		t.Errorf("Expected region 'us-central1', got %v", response["region"])
	}
}

func TestMintSessionVertexCredentials_MissingAuthHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/projects/:projectName/agentic-sessions/:sessionName/vertex/credentials", MintSessionVertexCredentials)

	// Request without Authorization header
	req := httptest.NewRequest("POST", "/api/projects/test-project/agentic-sessions/test-session/vertex/credentials", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestMintSessionVertexCredentials_InvalidToken(t *testing.T) {
	fakeClient := k8sfake.NewSimpleClientset()
	K8sClient = fakeClient

	// Mock TokenReview to return unauthenticated
	fakeClient.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, &authnv1.TokenReview{
			Status: authnv1.TokenReviewStatus{
				Authenticated: false,
				Error:         "invalid token",
			},
		}, nil
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/projects/:projectName/agentic-sessions/:sessionName/vertex/credentials", MintSessionVertexCredentials)

	req := httptest.NewRequest("POST", "/api/projects/test-project/agentic-sessions/test-session/vertex/credentials", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestMintSessionVertexCredentials_WrongNamespace(t *testing.T) {
	fakeClient := k8sfake.NewSimpleClientset()
	K8sClient = fakeClient

	// Mock TokenReview to return SA from different namespace
	fakeClient.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, &authnv1.TokenReview{
			Status: authnv1.TokenReviewStatus{
				Authenticated: true,
				User: authnv1.UserInfo{
					Username: "system:serviceaccount:wrong-namespace:test-session-runner",
				},
			},
		}, nil
	})

	sessionCR := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "vteam.ambient-code/v1alpha1",
			"kind":       "AgenticSession",
			"metadata": map[string]interface{}{
				"name":      "test-session",
				"namespace": "test-project",
				"annotations": map[string]interface{}{
					"ambient-code.io/runner-sa": "test-session-runner",
				},
			},
		},
	}

	s := runtime.NewScheme()
	DynamicClient = fake.NewSimpleDynamicClient(s, sessionCR)
	setupTestGVR()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/projects/:projectName/agentic-sessions/:sessionName/vertex/credentials", MintSessionVertexCredentials)

	req := httptest.NewRequest("POST", "/api/projects/test-project/agentic-sessions/test-session/vertex/credentials", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestMintSessionVertexCredentials_SessionNotFound(t *testing.T) {
	fakeClient := k8sfake.NewSimpleClientset()
	K8sClient = fakeClient

	// Mock valid TokenReview
	fakeClient.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, &authnv1.TokenReview{
			Status: authnv1.TokenReviewStatus{
				Authenticated: true,
				User: authnv1.UserInfo{
					Username: "system:serviceaccount:test-project:test-session-runner",
				},
			},
		}, nil
	})

	// Empty dynamic client (no session CR)
	s := runtime.NewScheme()
	DynamicClient = fake.NewSimpleDynamicClient(s)
	setupTestGVR()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/projects/:projectName/agentic-sessions/:sessionName/vertex/credentials", MintSessionVertexCredentials)

	req := httptest.NewRequest("POST", "/api/projects/test-project/agentic-sessions/test-session/vertex/credentials", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestMintSessionVertexCredentials_SAMismatch(t *testing.T) {
	fakeClient := k8sfake.NewSimpleClientset()
	K8sClient = fakeClient

	// Mock TokenReview with different SA name
	fakeClient.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, &authnv1.TokenReview{
			Status: authnv1.TokenReviewStatus{
				Authenticated: true,
				User: authnv1.UserInfo{
					Username: "system:serviceaccount:test-project:wrong-sa-name",
				},
			},
		}, nil
	})

	// Session expects different SA
	sessionCR := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "vteam.ambient-code/v1alpha1",
			"kind":       "AgenticSession",
			"metadata": map[string]interface{}{
				"name":      "test-session",
				"namespace": "test-project",
				"annotations": map[string]interface{}{
					"ambient-code.io/runner-sa": "expected-sa-name",
				},
			},
		},
	}

	s := runtime.NewScheme()
	DynamicClient = fake.NewSimpleDynamicClient(s, sessionCR)
	setupTestGVR()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/projects/:projectName/agentic-sessions/:sessionName/vertex/credentials", MintSessionVertexCredentials)

	req := httptest.NewRequest("POST", "/api/projects/test-project/agentic-sessions/test-session/vertex/credentials", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestMintSessionVertexCredentials_SecretNotFound(t *testing.T) {
	// Setup backend namespace
	Namespace = "backend-namespace"

	// K8s client with no vertex secret
	fakeClient := k8sfake.NewSimpleClientset()
	K8sClient = fakeClient

	fakeClient.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, &authnv1.TokenReview{
			Status: authnv1.TokenReviewStatus{
				Authenticated: true,
				User: authnv1.UserInfo{
					Username: "system:serviceaccount:test-project:test-session-runner",
				},
			},
		}, nil
	})

	sessionCR := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "vteam.ambient-code/v1alpha1",
			"kind":       "AgenticSession",
			"metadata": map[string]interface{}{
				"name":      "test-session",
				"namespace": "test-project",
				"annotations": map[string]interface{}{
					"ambient-code.io/runner-sa": "test-session-runner",
				},
			},
		},
	}

	s := runtime.NewScheme()
	DynamicClient = fake.NewSimpleDynamicClient(s, sessionCR)
	setupTestGVR()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/projects/:projectName/agentic-sessions/:sessionName/vertex/credentials", MintSessionVertexCredentials)

	req := httptest.NewRequest("POST", "/api/projects/test-project/agentic-sessions/test-session/vertex/credentials", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	if !contains(response["error"].(string), "Vertex credentials not configured") {
		t.Errorf("Expected error about missing credentials, got: %v", response["error"])
	}
}

func TestMintSessionVertexCredentials_MissingCredentialFile(t *testing.T) {
	// Setup backend namespace
	Namespace = "backend-namespace"

	// Secret missing the key file
	incompleteSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ambient-vertex",
			Namespace: "backend-namespace", // Must match Namespace variable
		},
		Data: map[string][]byte{
			"project-id": []byte("test-project"),
			// Missing ambient-code-key.json
		},
	}

	fakeClient := k8sfake.NewSimpleClientset(incompleteSecret)
	K8sClient = fakeClient

	fakeClient.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, &authnv1.TokenReview{
			Status: authnv1.TokenReviewStatus{
				Authenticated: true,
				User: authnv1.UserInfo{
					Username: "system:serviceaccount:test-project:test-session-runner",
				},
			},
		}, nil
	})

	sessionCR := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "vteam.ambient-code/v1alpha1",
			"kind":       "AgenticSession",
			"metadata": map[string]interface{}{
				"name":      "test-session",
				"namespace": "test-project",
				"annotations": map[string]interface{}{
					"ambient-code.io/runner-sa": "test-session-runner",
				},
			},
		},
	}

	s := runtime.NewScheme()
	DynamicClient = fake.NewSimpleDynamicClient(s, sessionCR)
	setupTestGVR()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/projects/:projectName/agentic-sessions/:sessionName/vertex/credentials", MintSessionVertexCredentials)

	req := httptest.NewRequest("POST", "/api/projects/test-project/agentic-sessions/test-session/vertex/credentials", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && stringContains(s, substr))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
