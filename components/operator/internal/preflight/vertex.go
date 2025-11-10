// Package preflight provides environment validation and configuration checks for the operator.
package preflight

import (
	"context"
	"log"

	"ambient-code-operator/internal/config"
	"ambient-code-operator/internal/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ValidateVertexConfig validates Vertex AI configuration at operator startup
func ValidateVertexConfig(operatorNamespace string) error {
	log.Printf("Vertex AI mode enabled - validating configuration...")

	// Note: Operator no longer needs Vertex environment variables or credentials
	// Runner fetches credentials from backend API using BOT_TOKEN authentication
	// Backend reads ambient-vertex secret from its own namespace

	// Check if ambient-vertex secret exists in backend namespace (informational only)
	// This is not a hard requirement as backend may be in different namespace
	_, err := config.K8sClient.CoreV1().Secrets(operatorNamespace).Get(
		context.TODO(),
		types.AmbientVertexSecretName,
		metav1.GetOptions{},
	)
	if err != nil {
		log.Printf("  Info: secret '%s' not found in namespace '%s'", types.AmbientVertexSecretName, operatorNamespace)
		log.Printf("  Note: Ensure backend namespace has the ambient-vertex secret with:")
		log.Printf("    kubectl create secret generic %s --from-file=ambient-code-key.json=/path/to/service-account.json --from-literal=project-id=YOUR_PROJECT --from-literal=region=YOUR_REGION -n <backend-namespace>",
			types.AmbientVertexSecretName)
		log.Printf("  Runner will fetch credentials from backend API")
	} else {
		log.Printf("  Secret '%s' found in namespace '%s' - backend can serve credentials to runners", types.AmbientVertexSecretName, operatorNamespace)
	}

	log.Printf("Vertex AI configuration validated successfully")
	return nil
}
