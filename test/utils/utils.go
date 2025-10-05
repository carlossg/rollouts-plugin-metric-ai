/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2" // nolint:revive,staticcheck
)

const (
	// prometheusOperatorVersion = "v0.77.1"
	// prometheusOperatorURL     = "https://github.com/prometheus-operator/prometheus-operator/" +
	// 	"releases/download/%s/bundle.yaml"

	ArgoRolloutsNamespace = "argo-rollouts"
)

func warnError(err error) {
	_, _ = fmt.Fprintf(GinkgoWriter, "warning: %v\n", err)
}

// Run executes the provided command within this context
func Run(cmd *exec.Cmd) (string, error) {
	dir, _ := GetProjectDir()
	cmd.Dir = dir

	if err := os.Chdir(cmd.Dir); err != nil {
		_, _ = fmt.Fprintf(GinkgoWriter, "chdir dir: %q\n", err)
	}

	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	command := strings.Join(cmd.Args, " ")
	_, _ = fmt.Fprintf(GinkgoWriter, "running: %q\n", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%q failed with error %q: %w", command, string(output), err)
	}

	return string(output), nil
}

// InstallPrometheusOperator installs the prometheus Operator to be used to export the enabled metrics.
// func InstallPrometheusOperator() error {
// 	url := fmt.Sprintf(prometheusOperatorURL, prometheusOperatorVersion)
// 	cmd := exec.Command("kubectl", "create", "-f", url)
// 	_, err := Run(cmd)
// 	return err
// }

// UninstallPrometheusOperator uninstalls the prometheus
// func UninstallPrometheusOperator() {
// 	url := fmt.Sprintf(prometheusOperatorURL, prometheusOperatorVersion)
// 	cmd := exec.Command("kubectl", "delete", "-f", url)
// 	if _, err := Run(cmd); err != nil {
// 		warnError(err)
// 	}
// }

// // IsPrometheusCRDsInstalled checks if any Prometheus CRDs are installed
// // by verifying the existence of key CRDs related to Prometheus.
// func IsPrometheusCRDsInstalled() bool {
// 	// List of common Prometheus CRDs
// 	prometheusCRDs := []string{
// 		"prometheuses.monitoring.coreos.com",
// 		"prometheusrules.monitoring.coreos.com",
// 		"prometheusagents.monitoring.coreos.com",
// 	}

// 	cmd := exec.Command("kubectl", "get", "crds", "-o", "custom-columns=NAME:.metadata.name")
// 	output, err := Run(cmd)
// 	if err != nil {
// 		return false
// 	}
// 	crdList := GetNonEmptyLines(output)
// 	for _, crd := range prometheusCRDs {
// 		for _, line := range crdList {
// 			if strings.Contains(line, crd) {
// 				return true
// 			}
// 		}
// 	}

// 	return false
// }

// UninstallArgoRollouts uninstalls the cert manager
func UninstallArgoRollouts() {
	cmd := exec.Command("kubectl", "delete", "-k", "config/argo-rollouts")
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}
}

// InstallArgoRollouts installs the cert manager bundle.
func InstallArgoRollouts() error {
	cmd := exec.Command("kubectl", "create", "namespace", ArgoRolloutsNamespace)
	if _, err := Run(cmd); err != nil {
		if strings.Contains(err.Error(), "AlreadyExists") {
			_, _ = fmt.Fprintf(GinkgoWriter, "Namespace %s already exists. Skipping creation...\n", "argo-rollouts")
		} else {
			return err
		}
	}

	cmd = exec.Command("kubectl", "apply", "-n", ArgoRolloutsNamespace, "-k", "config/argo-rollouts")
	// if _, err := Run(cmd); err != nil {
	// 	return err
	// }

	// install the rollouts examples
	// cmd = exec.Command("kubectl", "apply", "-n", ArgoRolloutsNamespace, "-k", "config/rollouts-examples")
	// if _, err := Run(cmd); err != nil {
	// 	return err
	// }
	// Wait for cert-manager-webhook to be ready, which can take time if cert-manager
	// was re-installed after uninstalling on a cluster.
	// cmd = exec.Command("kubectl", "wait", "deployment.apps/cert-manager-webhook",
	// 	"--for", "condition=Available",
	// 	"--namespace", "cert-manager",
	// 	"--timeout", "5m",
	// )

	// configure the plugins
	// cmd = exec.Command("kubectl", "apply", "-n", ArgoRolloutsNamespace, "-f", "./config/argorollouts/argo-rollouts-config.yaml")

	_, err := Run(cmd)
	return err
}

// IsArgoRolloutsCRDsInstalled checks if any Argo Rollouts CRDs are installed
// by verifying the existence of key CRDs related to Cert Manager.
func IsArgoRolloutsCRDsInstalled() bool {
	// List of common Argo Rollouts CRDs
	argoRolloutsCRDs := []string{
		"rollouts.argoproj.io",
		"analysisruns.argoproj.io",
	}

	// Execute the kubectl command to get all CRDs
	cmd := exec.Command("kubectl", "get", "crds")
	output, err := Run(cmd)
	if err != nil {
		return false
	}

	// Check if any of the Cert Manager CRDs are present
	crdList := GetNonEmptyLines(output)
	for _, crd := range argoRolloutsCRDs {
		for _, line := range crdList {
			if strings.Contains(line, crd) {
				return true
			}
		}
	}

	return false
}

// LoadImageToKindClusterWithName loads a local docker image to the kind cluster
func LoadImageToKindClusterWithName(name string) error {
	cluster := "kind"
	if v, ok := os.LookupEnv("KIND_CLUSTER"); ok {
		cluster = v
	} else {
		// if cluster exists, use it, otherwise use the default "kind"
		default_cluster := "rollouts-plugin-metric-ai-test-e2e"
		if clusters, err := exec.Command("kind", "get", "clusters").Output(); err == nil {
			clusters := strings.Split(string(clusters), "\n")
			for _, c := range clusters {
				if c == default_cluster {
					cluster = default_cluster
					break
				}
			}
		}
	}
	kindOptions := []string{"load", "docker-image", name, "--name", cluster}
	cmd := exec.Command("kind", kindOptions...)
	_, err := Run(cmd)
	return err
}

// GetNonEmptyLines converts given command output string into individual objects
// according to line breakers, and ignores the empty elements in it.
func GetNonEmptyLines(output string) []string {
	var res []string
	elements := strings.Split(output, "\n")
	for _, element := range elements {
		if element != "" {
			res = append(res, element)
		}
	}

	return res
}

// GetProjectDir will return the directory where the project is
func GetProjectDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return wd, fmt.Errorf("failed to get current working directory: %w", err)
	}
	wd = strings.ReplaceAll(wd, "/test/e2e", "")
	return wd, nil
}

// UncommentCode searches for target in the file and remove the comment prefix
// of the target content. The target content may span multiple lines.
func UncommentCode(filename, target, prefix string) error {
	// false positive
	// nolint:gosec
	content, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file %q: %w", filename, err)
	}
	strContent := string(content)

	idx := strings.Index(strContent, target)
	if idx < 0 {
		return fmt.Errorf("unable to find the code %q to be uncomment", target)
	}

	out := new(bytes.Buffer)
	_, err = out.Write(content[:idx])
	if err != nil {
		return fmt.Errorf("failed to write to output: %w", err)
	}

	scanner := bufio.NewScanner(bytes.NewBufferString(target))
	if !scanner.Scan() {
		return nil
	}
	for {
		if _, err = out.WriteString(strings.TrimPrefix(scanner.Text(), prefix)); err != nil {
			return fmt.Errorf("failed to write to output: %w", err)
		}
		// Avoid writing a newline in case the previous line was the last in target.
		if !scanner.Scan() {
			break
		}
		if _, err = out.WriteString("\n"); err != nil {
			return fmt.Errorf("failed to write to output: %w", err)
		}
	}

	if _, err = out.Write(content[idx+len(target):]); err != nil {
		return fmt.Errorf("failed to write to output: %w", err)
	}

	// false positive
	// nolint:gosec
	if err = os.WriteFile(filename, out.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write file %q: %w", filename, err)
	}

	return nil
}

// RestartArgoRollouts restarts the Argo Rollouts controller deployment
func RestartArgoRollouts() error {
	By("restarting Argo Rollouts controller")
	cmd := exec.Command("kubectl", "rollout", "restart", "deployment/argo-rollouts", "-n", ArgoRolloutsNamespace)
	_, err := Run(cmd)
	if err != nil {
		return err
	}

	By("waiting for Argo Rollouts rollout to complete")
	cmd = exec.Command("kubectl", "rollout", "status", "deployment/argo-rollouts", "-n", ArgoRolloutsNamespace, "--timeout=5m")
	_, err = Run(cmd)
	return err
}
