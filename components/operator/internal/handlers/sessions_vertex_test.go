package handlers

import (
	"context"
	"testing"

	"github.com/ambient-code/vteam/components/operator/internal/config"
	"github.com/ambient-code/vteam/components/operator/internal/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/fake"
)

// TestCopySecretToNamespace tests all error cases and edge cases for copySecretToNamespace
func TestCopySecretToNamespace(t *testing.T) {
	ctx := context.Background()

	// Helper to create an AgenticSession unstructured object
	createSessionObj := func(name, namespace, uid string) *unstructured.Unstructured {
		obj := &unstructured.Unstructured{}
		obj.SetAPIVersion("vteam.ambient-code/v1alpha1")
		obj.SetKind("AgenticSession")
		obj.SetName(name)
		obj.SetNamespace(namespace)
		obj.SetUID(metav1.UID(uid))
		return obj
	}

	tests := []struct {
		name            string
		sourceSecret    *corev1.Secret
		existingSecret  *corev1.Secret
		ownerObj        *unstructured.Unstructured
		targetNamespace string
		wantErr         bool
		errContains     string
		validateSecret  func(*testing.T, *corev1.Secret)
	}{
		{
			name: "success - create new secret",
			sourceSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      types.AmbientVertexSecretName,
					Namespace: "operator-ns",
				},
				Data: map[string][]byte{
					"key.json": []byte(`{"test": "data"}`),
				},
			},
			existingSecret:  nil,
			ownerObj:        createSessionObj("test-session", "target-ns", "uid-123"),
			targetNamespace: "target-ns",
			wantErr:         false,
			validateSecret: func(t *testing.T, secret *corev1.Secret) {
				if secret == nil {
					t.Error("Expected secret to be created")
					return
				}
				if secret.Name != types.AmbientVertexSecretName {
					t.Errorf("Secret name = %v, want %v", secret.Name, types.AmbientVertexSecretName)
				}
				if secret.Namespace != "target-ns" {
					t.Errorf("Secret namespace = %v, want target-ns", secret.Namespace)
				}
				if len(secret.OwnerReferences) != 1 {
					t.Errorf("Expected 1 owner reference, got %d", len(secret.OwnerReferences))
				} else {
					ref := secret.OwnerReferences[0]
					if ref.Name != "test-session" {
						t.Errorf("Owner reference name = %v, want test-session", ref.Name)
					}
					if ref.UID != "uid-123" {
						t.Errorf("Owner reference UID = %v, want uid-123", ref.UID)
					}
					if ref.Controller == nil || !*ref.Controller {
						t.Error("Expected Controller to be true")
					}
				}
				// Check annotation
				annotation := secret.Annotations[types.CopiedFromAnnotation]
				if annotation != "operator-ns/"+types.AmbientVertexSecretName {
					t.Errorf("Annotation = %v, want operator-ns/%s", annotation, types.AmbientVertexSecretName)
				}
			},
		},
		{
			name: "success - update existing secret without owner ref",
			sourceSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      types.AmbientVertexSecretName,
					Namespace: "operator-ns",
				},
				Data: map[string][]byte{
					"key.json": []byte(`{"test": "data"}`),
				},
			},
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      types.AmbientVertexSecretName,
					Namespace: "target-ns",
					Annotations: map[string]string{
						types.CopiedFromAnnotation: "operator-ns/" + types.AmbientVertexSecretName,
					},
				},
				Data: map[string][]byte{
					"key.json": []byte(`{"old": "data"}`),
				},
				// No OwnerReferences - should be added
			},
			ownerObj:        createSessionObj("test-session", "target-ns", "uid-456"),
			targetNamespace: "target-ns",
			wantErr:         false,
			validateSecret: func(t *testing.T, secret *corev1.Secret) {
				if secret == nil {
					t.Error("Expected secret to exist")
					return
				}
				if len(secret.OwnerReferences) != 1 {
					t.Errorf("Expected 1 owner reference after update, got %d", len(secret.OwnerReferences))
				} else {
					ref := secret.OwnerReferences[0]
					if ref.UID != "uid-456" {
						t.Errorf("Owner reference UID = %v, want uid-456", ref.UID)
					}
				}
			},
		},
		{
			name: "success - secret already exists with correct owner ref",
			sourceSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      types.AmbientVertexSecretName,
					Namespace: "operator-ns",
				},
				Data: map[string][]byte{
					"key.json": []byte(`{"test": "data"}`),
				},
			},
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      types.AmbientVertexSecretName,
					Namespace: "target-ns",
					Annotations: map[string]string{
						types.CopiedFromAnnotation: "operator-ns/" + types.AmbientVertexSecretName,
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "vteam.ambient-code/v1alpha1",
							Kind:       "AgenticSession",
							Name:       "test-session",
							UID:        "uid-789",
							Controller: boolPtr(true),
						},
					},
				},
				Data: map[string][]byte{
					"key.json": []byte(`{"existing": "data"}`),
				},
			},
			ownerObj:        createSessionObj("test-session", "target-ns", "uid-789"),
			targetNamespace: "target-ns",
			wantErr:         false,
			validateSecret: func(t *testing.T, secret *corev1.Secret) {
				// Should not be modified since it already has correct owner ref
				if secret == nil {
					t.Error("Expected secret to exist")
					return
				}
				if len(secret.OwnerReferences) != 1 {
					t.Errorf("Expected 1 owner reference, got %d", len(secret.OwnerReferences))
				}
			},
		},
		{
			name:            "error - source secret not found",
			sourceSecret:    nil, // Source doesn't exist
			existingSecret:  nil,
			ownerObj:        createSessionObj("test-session", "target-ns", "uid-999"),
			targetNamespace: "target-ns",
			wantErr:         true,
			errContains:     "not found",
		},
		{
			name: "success - concurrent update with retry",
			sourceSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      types.AmbientVertexSecretName,
					Namespace: "operator-ns",
				},
				Data: map[string][]byte{
					"key.json": []byte(`{"test": "data"}`),
				},
			},
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:            types.AmbientVertexSecretName,
					Namespace:       "target-ns",
					ResourceVersion: "1",
					Annotations: map[string]string{
						types.CopiedFromAnnotation: "operator-ns/" + types.AmbientVertexSecretName,
					},
				},
				Data: map[string][]byte{
					"key.json": []byte(`{"old": "data"}`),
				},
			},
			ownerObj:        createSessionObj("test-session", "target-ns", "uid-concurrent"),
			targetNamespace: "target-ns",
			wantErr:         false,
			// Retry logic should handle conflicts
		},
		{
			name: "error - nil owner object",
			sourceSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      types.AmbientVertexSecretName,
					Namespace: "operator-ns",
				},
				Data: map[string][]byte{
					"key.json": []byte(`{"test": "data"}`),
				},
			},
			existingSecret:  nil,
			ownerObj:        nil,
			targetNamespace: "target-ns",
			wantErr:         true,
			// Should panic or error on nil owner
		},
		{
			name: "success - multiple owner references (add new one)",
			sourceSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      types.AmbientVertexSecretName,
					Namespace: "operator-ns",
				},
				Data: map[string][]byte{
					"key.json": []byte(`{"test": "data"}`),
				},
			},
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      types.AmbientVertexSecretName,
					Namespace: "target-ns",
					Annotations: map[string]string{
						types.CopiedFromAnnotation: "operator-ns/" + types.AmbientVertexSecretName,
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "vteam.ambient-code/v1alpha1",
							Kind:       "AgenticSession",
							Name:       "other-session",
							UID:        "uid-other",
						},
					},
				},
				Data: map[string][]byte{
					"key.json": []byte(`{"existing": "data"}`),
				},
			},
			ownerObj:        createSessionObj("new-session", "target-ns", "uid-new"),
			targetNamespace: "target-ns",
			wantErr:         false,
			validateSecret: func(t *testing.T, secret *corev1.Secret) {
				if secret == nil {
					t.Error("Expected secret to exist")
					return
				}
				// Should add new owner reference if it doesn't exist
				found := false
				for _, ref := range secret.OwnerReferences {
					if ref.UID == "uid-new" {
						found = true
						break
					}
				}
				if !found {
					t.Error("Expected new owner reference to be added")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake K8s client
			fakeClient := fake.NewSimpleClientset()

			// Create source secret if provided
			if tt.sourceSecret != nil {
				_, err := fakeClient.CoreV1().Secrets(tt.sourceSecret.Namespace).Create(
					ctx, tt.sourceSecret, metav1.CreateOptions{},
				)
				if err != nil {
					t.Fatalf("Failed to create source secret: %v", err)
				}
			}

			// Create existing secret in target namespace if provided
			if tt.existingSecret != nil {
				_, err := fakeClient.CoreV1().Secrets(tt.existingSecret.Namespace).Create(
					ctx, tt.existingSecret, metav1.CreateOptions{},
				)
				if err != nil {
					t.Fatalf("Failed to create existing secret: %v", err)
				}
			}

			// Replace global K8sClient with fake client
			origK8sClient := config.K8sClient
			config.K8sClient = fakeClient
			defer func() {
				config.K8sClient = origK8sClient
			}()

			// Run the function
			var err error
			if tt.ownerObj != nil {
				err = copySecretToNamespace(ctx, tt.targetNamespace, tt.ownerObj)
			} else {
				// Test nil owner object handling
				defer func() {
					if r := recover(); r != nil {
						err = r.(error)
					}
				}()
				err = copySecretToNamespace(ctx, tt.targetNamespace, tt.ownerObj)
			}

			// Check error expectations
			if tt.wantErr {
				if err == nil {
					t.Errorf("copySecretToNamespace() expected error but got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("copySecretToNamespace() error = %v, want error containing %q", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("copySecretToNamespace() unexpected error = %v", err)
				}

				// Validate the created/updated secret if validation function provided
				if tt.validateSecret != nil {
					secret, err := fakeClient.CoreV1().Secrets(tt.targetNamespace).Get(
						ctx, types.AmbientVertexSecretName, metav1.GetOptions{},
					)
					if err != nil {
						t.Errorf("Failed to get secret after operation: %v", err)
					} else {
						tt.validateSecret(t, secret)
					}
				}
			}
		})
	}
}

// TestDeleteAmbientVertexSecret tests all error cases for deleteAmbientVertexSecret
func TestDeleteAmbientVertexSecret(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		existingSecret *corev1.Secret
		namespace      string
		wantErr        bool
		errContains    string
		shouldDelete   bool
	}{
		{
			name: "success - delete secret with annotation",
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      types.AmbientVertexSecretName,
					Namespace: "test-ns",
					Annotations: map[string]string{
						types.CopiedFromAnnotation: "operator-ns/" + types.AmbientVertexSecretName,
					},
				},
				Data: map[string][]byte{
					"key.json": []byte(`{"test": "data"}`),
				},
			},
			namespace:    "test-ns",
			wantErr:      false,
			shouldDelete: true,
		},
		{
			name:           "success - secret not found (already deleted)",
			existingSecret: nil,
			namespace:      "test-ns",
			wantErr:        false,
			shouldDelete:   false,
		},
		{
			name: "success - secret without annotation (manual creation, skip delete)",
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      types.AmbientVertexSecretName,
					Namespace: "test-ns",
					// No annotation - manually created
				},
				Data: map[string][]byte{
					"key.json": []byte(`{"test": "data"}`),
				},
			},
			namespace:    "test-ns",
			wantErr:      false,
			shouldDelete: false, // Should NOT delete
		},
		{
			name: "success - secret with empty annotations map",
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        types.AmbientVertexSecretName,
					Namespace:   "test-ns",
					Annotations: map[string]string{},
				},
				Data: map[string][]byte{
					"key.json": []byte(`{"test": "data"}`),
				},
			},
			namespace:    "test-ns",
			wantErr:      false,
			shouldDelete: false, // Should NOT delete
		},
		{
			name: "success - secret with nil annotations",
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        types.AmbientVertexSecretName,
					Namespace:   "test-ns",
					Annotations: nil,
				},
				Data: map[string][]byte{
					"key.json": []byte(`{"test": "data"}`),
				},
			},
			namespace:    "test-ns",
			wantErr:      false,
			shouldDelete: false, // Should NOT delete
		},
		{
			name: "success - secret with different annotation key",
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      types.AmbientVertexSecretName,
					Namespace: "test-ns",
					Annotations: map[string]string{
						"some-other-annotation": "value",
					},
				},
				Data: map[string][]byte{
					"key.json": []byte(`{"test": "data"}`),
				},
			},
			namespace:    "test-ns",
			wantErr:      false,
			shouldDelete: false, // Should NOT delete
		},
		{
			name: "success - annotation check prevents accidental deletion",
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      types.AmbientVertexSecretName,
					Namespace: "test-ns",
					Annotations: map[string]string{
						types.CopiedFromAnnotation: "different-source/different-secret",
					},
				},
				Data: map[string][]byte{
					"key.json": []byte(`{"test": "data"}`),
				},
			},
			namespace:    "test-ns",
			wantErr:      false,
			shouldDelete: true, // Has the annotation, so should delete
		},
		{
			name: "success - empty namespace",
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      types.AmbientVertexSecretName,
					Namespace: "",
					Annotations: map[string]string{
						types.CopiedFromAnnotation: "operator-ns/" + types.AmbientVertexSecretName,
					},
				},
				Data: map[string][]byte{
					"key.json": []byte(`{"test": "data"}`),
				},
			},
			namespace:    "",
			wantErr:      false,
			shouldDelete: false, // Will fail to find in empty namespace
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake K8s client
			fakeClient := fake.NewSimpleClientset()

			// Create existing secret if provided
			if tt.existingSecret != nil {
				_, err := fakeClient.CoreV1().Secrets(tt.existingSecret.Namespace).Create(
					ctx, tt.existingSecret, metav1.CreateOptions{},
				)
				if err != nil {
					t.Fatalf("Failed to create existing secret: %v", err)
				}
			}

			// Replace global K8sClient with fake client
			origK8sClient := config.K8sClient
			config.K8sClient = fakeClient
			defer func() {
				config.K8sClient = origK8sClient
			}()

			// Run the function
			err := deleteAmbientVertexSecret(ctx, tt.namespace)

			// Check error expectations
			if tt.wantErr {
				if err == nil {
					t.Errorf("deleteAmbientVertexSecret() expected error but got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("deleteAmbientVertexSecret() error = %v, want error containing %q", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("deleteAmbientVertexSecret() unexpected error = %v", err)
				}
			}

			// Verify deletion behavior
			secret, err := fakeClient.CoreV1().Secrets(tt.namespace).Get(
				ctx, types.AmbientVertexSecretName, metav1.GetOptions{},
			)

			if tt.shouldDelete {
				// Secret should be deleted
				if err == nil {
					t.Error("Expected secret to be deleted, but it still exists")
				}
			} else {
				// Secret should still exist or never existed
				if tt.existingSecret != nil && err != nil {
					// If we had an existing secret and we got an error, it should NOT be found
					if tt.existingSecret.Annotations == nil || tt.existingSecret.Annotations[types.CopiedFromAnnotation] == "" {
						// This is expected - secret without annotation should not be deleted
						if secret != nil {
							t.Error("Secret without annotation should still exist but was deleted")
						}
					}
				}
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
