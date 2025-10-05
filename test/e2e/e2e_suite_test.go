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

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/argoproj-labs/rollouts-plugin-metric-ai/test/utils"
)

var (
	// Optional Environment Variables:
	// - ARGO_ROLLOUTS_INSTALL_SKIP=true: Skips Argo Rollouts installation during test setup.
	// These variables are useful if Argo Rollouts is already installed, avoiding
	// re-installation and conflicts.
	skipArgoRolloutsInstall = os.Getenv("ARGO_ROLLOUTS_INSTALL_SKIP") == "true"
	// isArgoRolloutsAlreadyInstalled will be set true when Argo Rollouts CRDs be found on the cluster
	isArgoRolloutsAlreadyInstalled = false

	// projectImage is the name of the image which will be build and loaded
	// with the code source changes to be tested.
	projectImage = "csanchez/rollouts-plugin-metric-ai:latest"
)

// TestE2E runs the end-to-end (e2e) test suite for the project. These tests execute in an isolated,
// temporary environment to validate project changes with the purposed to be used in CI jobs.
// The default setup requires Kind, builds/loads the Manager Docker image locally, and installs
// Argo Rollouts.
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting kubebuilder-example integration test suite\n")
	RunSpecs(t, "e2e suite")
}

var _ = BeforeSuite(func() {
	By("building the image")
	cmd := exec.Command("make", "docker-build", fmt.Sprintf("IMG=%s", projectImage))
	_, err := utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to build the manager(Operator) image")

	// TODO(user): If you want to change the e2e test vendor from Kind, ensure the image is
	// built and available before running the tests. Also, remove the following block.
	By("loading the image on Kind")
	err = utils.LoadImageToKindClusterWithName(projectImage)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to load the manager(Operator) image into Kind")

	// The tests-e2e are intended to run on a temporary cluster that is created and destroyed for testing.
	// To prevent errors when tests run in environments with Argo Rollouts already installed,
	// we check for its presence before execution.
	// Setup Argo Rollouts before the suite if not skipped and if not already installed
	if !skipArgoRolloutsInstall {
		By("checking if Argo Rollouts is installed already")
		isArgoRolloutsAlreadyInstalled = utils.IsArgoRolloutsCRDsInstalled()
		// if !isArgoRolloutsAlreadyInstalled {
		_, _ = fmt.Fprintf(GinkgoWriter, "Installing Argo Rollouts...\n")
		Expect(utils.InstallArgoRollouts()).To(Succeed(), "Failed to install Argo Rollouts")

		By("restarting Argo Rollouts controller after installation")
		Expect(utils.RestartArgoRollouts()).To(Succeed(), "Failed to restart Argo Rollouts controller")

		// } else {
		// 	_, _ = fmt.Fprintf(GinkgoWriter, "WARNING: Argo Rollouts is already installed. Skipping installation...\n")
		// }
	}
})

var _ = AfterSuite(func() {
	// Teardown Argo Rollouts after the suite if not skipped and if it was not already installed
	// if !skipArgoRolloutsInstall && !isArgoRolloutsAlreadyInstalled {
	// 	_, _ = fmt.Fprintf(GinkgoWriter, "Uninstalling Argo Rollouts...\n")
	// 	utils.UninstallArgoRollouts()
	// }
})
