package preflight

import (
	"os"
	"testing"

	"github.com/ambient-code/vteam/components/operator/internal/config"
	"github.com/ambient-code/vteam/components/operator/internal/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// TestValidateVertexConfig tests all error cases for ValidateVertexConfig
func TestValidateVertexConfig(t *testing.T) {
	tests := []struct {
		name           string
		envVars        map[string]string
		existingSecret *corev1.Secret
		wantErr        bool
		errContains    string
		setupK8sClient bool
	}{
		{
			name: "success - all valid",
			envVars: map[string]string{
				"ANTHROPIC_VERTEX_PROJECT_ID":    "test-project-123",
				"CLOUD_ML_REGION":                "us-central1",
				"GOOGLE_APPLICATION_CREDENTIALS": "/path/to/creds.json",
				"OPERATOR_NAMESPACE":             "test-namespace",
			},
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      types.AmbientVertexSecretName,
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"key.json": []byte(`{
						"type": "service_account",
						"project_id": "test-project-123",
						"private_key": "-----BEGIN PRIVATE KEY-----\nMIIE...\n-----END PRIVATE KEY-----\n",
						"client_email": "test@test-project-123.iam.gserviceaccount.com"
					}`),
				},
			},
			setupK8sClient: true,
			wantErr:        false,
		},
		{
			name: "error - missing ANTHROPIC_VERTEX_PROJECT_ID",
			envVars: map[string]string{
				"CLOUD_ML_REGION":                "us-central1",
				"GOOGLE_APPLICATION_CREDENTIALS": "/path/to/creds.json",
				"OPERATOR_NAMESPACE":             "test-namespace",
			},
			setupK8sClient: true,
			wantErr:        true,
			errContains:    "ANTHROPIC_VERTEX_PROJECT_ID is not set",
		},
		{
			name: "error - missing CLOUD_ML_REGION",
			envVars: map[string]string{
				"ANTHROPIC_VERTEX_PROJECT_ID":    "test-project-123",
				"GOOGLE_APPLICATION_CREDENTIALS": "/path/to/creds.json",
				"OPERATOR_NAMESPACE":             "test-namespace",
			},
			setupK8sClient: true,
			wantErr:        true,
			errContains:    "CLOUD_ML_REGION is not set",
		},
		{
			name: "error - missing GOOGLE_APPLICATION_CREDENTIALS",
			envVars: map[string]string{
				"ANTHROPIC_VERTEX_PROJECT_ID": "test-project-123",
				"CLOUD_ML_REGION":             "us-central1",
				"OPERATOR_NAMESPACE":          "test-namespace",
			},
			setupK8sClient: true,
			wantErr:        true,
			errContains:    "GOOGLE_APPLICATION_CREDENTIALS is not set",
		},
		{
			name: "error - empty ANTHROPIC_VERTEX_PROJECT_ID",
			envVars: map[string]string{
				"ANTHROPIC_VERTEX_PROJECT_ID":    "",
				"CLOUD_ML_REGION":                "us-central1",
				"GOOGLE_APPLICATION_CREDENTIALS": "/path/to/creds.json",
				"OPERATOR_NAMESPACE":             "test-namespace",
			},
			setupK8sClient: true,
			wantErr:        true,
			errContains:    "ANTHROPIC_VERTEX_PROJECT_ID is not set",
		},
		{
			name: "error - secret not found",
			envVars: map[string]string{
				"ANTHROPIC_VERTEX_PROJECT_ID":    "test-project-123",
				"CLOUD_ML_REGION":                "us-central1",
				"GOOGLE_APPLICATION_CREDENTIALS": "/path/to/creds.json",
				"OPERATOR_NAMESPACE":             "test-namespace",
			},
			existingSecret: nil, // No secret created
			setupK8sClient: true,
			wantErr:        true,
			errContains:    "not found",
		},
		{
			name: "error - secret missing key.json field",
			envVars: map[string]string{
				"ANTHROPIC_VERTEX_PROJECT_ID":    "test-project-123",
				"CLOUD_ML_REGION":                "us-central1",
				"GOOGLE_APPLICATION_CREDENTIALS": "/path/to/creds.json",
				"OPERATOR_NAMESPACE":             "test-namespace",
			},
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      types.AmbientVertexSecretName,
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"wrong-key": []byte(`{}`),
				},
			},
			setupK8sClient: true,
			wantErr:        true,
			errContains:    "missing 'key.json'",
		},
		{
			name: "error - invalid JSON in key.json",
			envVars: map[string]string{
				"ANTHROPIC_VERTEX_PROJECT_ID":    "test-project-123",
				"CLOUD_ML_REGION":                "us-central1",
				"GOOGLE_APPLICATION_CREDENTIALS": "/path/to/creds.json",
				"OPERATOR_NAMESPACE":             "test-namespace",
			},
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      types.AmbientVertexSecretName,
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"key.json": []byte(`{invalid json`),
				},
			},
			setupK8sClient: true,
			wantErr:        true,
			errContains:    "failed to parse JSON",
		},
		{
			name: "error - missing type field",
			envVars: map[string]string{
				"ANTHROPIC_VERTEX_PROJECT_ID":    "test-project-123",
				"CLOUD_ML_REGION":                "us-central1",
				"GOOGLE_APPLICATION_CREDENTIALS": "/path/to/creds.json",
				"OPERATOR_NAMESPACE":             "test-namespace",
			},
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      types.AmbientVertexSecretName,
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"key.json": []byte(`{
						"project_id": "test-project-123",
						"private_key": "key",
						"client_email": "test@test.com"
					}`),
				},
			},
			setupK8sClient: true,
			wantErr:        true,
			errContains:    "missing 'type' field",
		},
		{
			name: "error - missing project_id field",
			envVars: map[string]string{
				"ANTHROPIC_VERTEX_PROJECT_ID":    "test-project-123",
				"CLOUD_ML_REGION":                "us-central1",
				"GOOGLE_APPLICATION_CREDENTIALS": "/path/to/creds.json",
				"OPERATOR_NAMESPACE":             "test-namespace",
			},
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      types.AmbientVertexSecretName,
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"key.json": []byte(`{
						"type": "service_account",
						"private_key": "key",
						"client_email": "test@test.com"
					}`),
				},
			},
			setupK8sClient: true,
			wantErr:        true,
			errContains:    "missing 'project_id' field",
		},
		{
			name: "error - missing private_key field",
			envVars: map[string]string{
				"ANTHROPIC_VERTEX_PROJECT_ID":    "test-project-123",
				"CLOUD_ML_REGION":                "us-central1",
				"GOOGLE_APPLICATION_CREDENTIALS": "/path/to/creds.json",
				"OPERATOR_NAMESPACE":             "test-namespace",
			},
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      types.AmbientVertexSecretName,
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"key.json": []byte(`{
						"type": "service_account",
						"project_id": "test-project-123",
						"client_email": "test@test.com"
					}`),
				},
			},
			setupK8sClient: true,
			wantErr:        true,
			errContains:    "missing 'private_key' field",
		},
		{
			name: "error - missing client_email field",
			envVars: map[string]string{
				"ANTHROPIC_VERTEX_PROJECT_ID":    "test-project-123",
				"CLOUD_ML_REGION":                "us-central1",
				"GOOGLE_APPLICATION_CREDENTIALS": "/path/to/creds.json",
				"OPERATOR_NAMESPACE":             "test-namespace",
			},
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      types.AmbientVertexSecretName,
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"key.json": []byte(`{
						"type": "service_account",
						"project_id": "test-project-123",
						"private_key": "key"
					}`),
				},
			},
			setupK8sClient: true,
			wantErr:        true,
			errContains:    "missing 'client_email' field",
		},
		{
			name: "error - wrong type value",
			envVars: map[string]string{
				"ANTHROPIC_VERTEX_PROJECT_ID":    "test-project-123",
				"CLOUD_ML_REGION":                "us-central1",
				"GOOGLE_APPLICATION_CREDENTIALS": "/path/to/creds.json",
				"OPERATOR_NAMESPACE":             "test-namespace",
			},
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      types.AmbientVertexSecretName,
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"key.json": []byte(`{
						"type": "user_account",
						"project_id": "test-project-123",
						"private_key": "key",
						"client_email": "test@test.com"
					}`),
				},
			},
			setupK8sClient: true,
			wantErr:        true,
			errContains:    "expected 'type' to be 'service_account'",
		},
		{
			name: "success - project_id mismatch with warning (non-fatal)",
			envVars: map[string]string{
				"ANTHROPIC_VERTEX_PROJECT_ID":    "env-project-id",
				"CLOUD_ML_REGION":                "us-central1",
				"GOOGLE_APPLICATION_CREDENTIALS": "/path/to/creds.json",
				"OPERATOR_NAMESPACE":             "test-namespace",
			},
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      types.AmbientVertexSecretName,
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"key.json": []byte(`{
						"type": "service_account",
						"project_id": "secret-project-id",
						"private_key": "-----BEGIN PRIVATE KEY-----\nMIIE...\n-----END PRIVATE KEY-----\n",
						"client_email": "test@secret-project-id.iam.gserviceaccount.com"
					}`),
				},
			},
			setupK8sClient: true,
			wantErr:        false,
			// Note: This should log a warning but not fail
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original env vars
			origEnv := make(map[string]string)
			envKeys := []string{"ANTHROPIC_VERTEX_PROJECT_ID", "CLOUD_ML_REGION", "GOOGLE_APPLICATION_CREDENTIALS", "OPERATOR_NAMESPACE"}
			for _, key := range envKeys {
				origEnv[key] = os.Getenv(key)
			}

			// Restore env vars after test
			defer func() {
				for key, val := range origEnv {
					if val == "" {
						os.Unsetenv(key)
					} else {
						os.Setenv(key, val)
					}
				}
			}()

			// Clear all env vars first
			for _, key := range envKeys {
				os.Unsetenv(key)
			}

			// Set test env vars
			for key, val := range tt.envVars {
				if val != "" {
					os.Setenv(key, val)
				}
			}

			// Setup fake K8s client if needed
			if tt.setupK8sClient {
				fakeClient := fake.NewSimpleClientset()
				if tt.existingSecret != nil {
					_, err := fakeClient.CoreV1().Secrets(tt.existingSecret.Namespace).Create(
						metav1.CreateOptions{}, tt.existingSecret,
					)
					if err != nil {
						t.Fatalf("Failed to create fake secret: %v", err)
					}
				}

				// Replace global K8sClient with fake client
				origK8sClient := config.K8sClient
				config.K8sClient = fakeClient
				defer func() {
					config.K8sClient = origK8sClient
				}()
			}

			// Run the function
			err := ValidateVertexConfig()

			// Check results
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateVertexConfig() expected error but got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("ValidateVertexConfig() error = %v, want error containing %q", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateVertexConfig() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestValidateVertexSecret tests the validateVertexSecret helper function
func TestValidateVertexSecret(t *testing.T) {
	tests := []struct {
		name        string
		secret      *corev1.Secret
		projectID   string
		wantErr     bool
		errContains string
	}{
		{
			name: "valid secret",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					"key.json": []byte(`{
						"type": "service_account",
						"project_id": "test-project",
						"private_key": "key",
						"client_email": "test@test.com"
					}`),
				},
			},
			projectID: "test-project",
			wantErr:   false,
		},
		{
			name: "empty secret data",
			secret: &corev1.Secret{
				Data: map[string][]byte{},
			},
			projectID:   "test-project",
			wantErr:     true,
			errContains: "missing 'key.json'",
		},
		{
			name: "nil secret data",
			secret: &corev1.Secret{
				Data: nil,
			},
			projectID:   "test-project",
			wantErr:     true,
			errContains: "missing 'key.json'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVertexSecret(tt.secret, tt.projectID)

			if tt.wantErr {
				if err == nil {
					t.Errorf("validateVertexSecret() expected error but got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("validateVertexSecret() error = %v, want error containing %q", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("validateVertexSecret() unexpected error = %v", err)
				}
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
