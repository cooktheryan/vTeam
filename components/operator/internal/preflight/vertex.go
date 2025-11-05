package preflight

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"ambient-code-operator/internal/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ValidateVertexConfig validates Vertex AI configuration at operator startup
func ValidateVertexConfig(operatorNamespace string) error {
	log.Printf("Vertex AI mode enabled - validating configuration...")

	// Check required environment variables
	requiredEnvVars := map[string]string{
		"ANTHROPIC_VERTEX_PROJECT_ID":   os.Getenv("ANTHROPIC_VERTEX_PROJECT_ID"),
		"CLOUD_ML_REGION":                os.Getenv("CLOUD_ML_REGION"),
		"GOOGLE_APPLICATION_CREDENTIALS": os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"),
	}

	for name, value := range requiredEnvVars {
		if value == "" {
			return fmt.Errorf("CLAUDE_CODE_USE_VERTEX=1 but %s is not set", name)
		}
		log.Printf("  %s: %s", name, value)
	}

	// Check that ambient-vertex secret exists in operator namespace
	secretName := "ambient-vertex"
	secret, err := config.K8sClient.CoreV1().Secrets(operatorNamespace).Get(
		context.TODO(),
		secretName,
		metav1.GetOptions{},
	)
	if err != nil {
		return fmt.Errorf("secret '%s' not found in namespace '%s': %w\n"+
			"Please create the secret with: kubectl create secret generic %s --from-file=key.json=/path/to/service-account.json -n %s",
			secretName, operatorNamespace, err, secretName, operatorNamespace)
	}
	log.Printf("  Secret '%s' found in namespace '%s'", secretName, operatorNamespace)

	// Validate secret structure
	if err := validateVertexSecret(secret, os.Getenv("ANTHROPIC_VERTEX_PROJECT_ID")); err != nil {
		return fmt.Errorf("secret '%s' is invalid: %w", secretName, err)
	}
	log.Printf("  Secret structure validated")

	log.Printf("Vertex AI configuration validated successfully")
	return nil
}

// validateVertexSecret validates the structure of a Vertex AI secret
func validateVertexSecret(secret *corev1.Secret, expectedProjectID string) error {
	if secret == nil {
		return fmt.Errorf("secret is nil")
	}

	// Check for key.json (standard naming)
	if secret.Data["key.json"] == nil {
		return fmt.Errorf("secret missing 'key.json' key - ensure secret was created with --from-file=key.json=/path/to/file")
	}

	// Validate it's valid JSON
	var data map[string]any
	if err := json.Unmarshal(secret.Data["key.json"], &data); err != nil {
		return fmt.Errorf("'key.json' is not valid JSON: %w", err)
	}

	// Validate it looks like a Google service account JSON
	requiredFields := []string{"type", "project_id", "private_key", "client_email"}
	for _, field := range requiredFields {
		if _, ok := data[field]; !ok {
			return fmt.Errorf("'key.json' missing required field '%s' - doesn't appear to be a valid service account key", field)
		}
	}

	// Validate type is service_account
	if typeVal, ok := data["type"].(string); !ok || typeVal != "service_account" {
		return fmt.Errorf("'key.json' type is '%v', expected 'service_account'", data["type"])
	}

	// Validate project_id matches configured value (if provided)
	if expectedProjectID != "" {
		if projectID, ok := data["project_id"].(string); ok {
			if projectID != expectedProjectID {
				log.Printf("  Warning: Service account project_id '%s' differs from ANTHROPIC_VERTEX_PROJECT_ID '%s'", projectID, expectedProjectID)
				log.Printf("  Warning: This may cause authentication failures if the service account is from a different project")
			}
		}
	}

	return nil
}
