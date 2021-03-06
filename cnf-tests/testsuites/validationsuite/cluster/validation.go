package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	igntypes "github.com/coreos/ignition/config/v2_2/types"
	corev1 "k8s.io/api/core/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	goclient "sigs.k8s.io/controller-runtime/pkg/client"

	sriovv1 "github.com/k8snetworkplumbingwg/sriov-network-operator/api/v1"
	clientmachineconfigv1 "github.com/openshift/machine-config-operator/pkg/apis/machineconfiguration.openshift.io/v1"

	sriovtestclient "github.com/k8snetworkplumbingwg/sriov-network-operator/test/util/client"
	testclient "github.com/openshift-kni/cnf-features-deploy/cnf-tests/testsuites/pkg/client"
	"github.com/openshift-kni/cnf-features-deploy/cnf-tests/testsuites/pkg/namespaces"
	utilNodes "github.com/openshift-kni/cnf-features-deploy/cnf-tests/testsuites/pkg/nodes"
	"github.com/openshift-kni/cnf-features-deploy/cnf-tests/testsuites/pkg/sriov"
	"github.com/openshift-kni/cnf-features-deploy/cnf-tests/testsuites/pkg/utils"
)

var (
	machineConfigPoolNodeSelector string
)

func init() {
	roleWorkerCNF := os.Getenv("ROLE_WORKER_CNF")
	if roleWorkerCNF == "" {
		roleWorkerCNF = "worker-cnf"
	}

	machineConfigPoolNodeSelector = fmt.Sprintf("node-role.kubernetes.io/%s", roleWorkerCNF)
}

var _ = Describe("validation", func() {
	Context("general", func() {
		It("should report all machine config pools are in ready status", func() {
			mcp := &clientmachineconfigv1.MachineConfigPoolList{}
			err := testclient.Client.List(context.TODO(), mcp)
			Expect(err).ToNot(HaveOccurred())

			for _, mcItem := range mcp.Items {
				Expect(mcItem.Status.MachineCount).To(Equal(mcItem.Status.ReadyMachineCount))
			}
		})

		It("should have one machine config pool with the requested label", func() {
			mcp := &clientmachineconfigv1.MachineConfigPoolList{}
			err := testclient.Client.List(context.TODO(), mcp)
			Expect(err).ToNot(HaveOccurred())

			mcpExist := false
			for _, mcpItem := range mcp.Items {
				if mcpItem.Spec.NodeSelector != nil {
					if _, exist := mcpItem.Spec.NodeSelector.MatchLabels[machineConfigPoolNodeSelector]; exist {
						mcpExist = exist
						break
					}
				}
			}

			Expect(mcpExist).To(BeTrue(), fmt.Sprintf("was not able to func a machine config pool with %s label", machineConfigPoolNodeSelector))
		})

		It("[ovn] should have a openshift-ovn-kubernetes namespace", func() {
			_, err := testclient.Client.Namespaces().Get(context.Background(), "openshift-ovn-kubernetes", metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			_, err = testclient.Client.Namespaces().Get(context.Background(), "openshift-sdn", metav1.GetOptions{})
			Expect(err).To(HaveOccurred())
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})

		It("should have all the nodes in ready", func() {
			nodes, err := testclient.Client.Nodes().List(context.Background(), metav1.ListOptions{})
			Expect(err).ToNot(HaveOccurred())
			filtered, err := utilNodes.MatchingOptionalSelector(nodes.Items)

			for _, node := range filtered {
				nodeReady := false
				for _, condition := range node.Status.Conditions {
					if condition.Type == corev1.NodeReady &&
						condition.Status == corev1.ConditionTrue {
						nodeReady = true
					}
				}
				Expect(nodeReady).To(BeTrue(), "Node ", node.Name, " is not ready")
			}
		})
	})

	Context("performance", func() {
		It("should have the performance operator namespace", func() {
			_, err := testclient.Client.Namespaces().Get(context.Background(), namespaces.PerformanceOperator, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should have the performance operator deployment in running state", func() {
			deploy, err := testclient.Client.Deployments(namespaces.PerformanceOperator).Get(context.Background(), utils.PerformanceOperatorDeploymentName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(deploy.Status.Replicas).To(Equal(deploy.Status.ReadyReplicas))

			pods, err := testclient.Client.Pods(namespaces.PerformanceOperator).List(context.Background(), metav1.ListOptions{
				LabelSelector: fmt.Sprintf("name=%s", utils.PerformanceOperatorDeploymentName)})
			Expect(err).ToNot(HaveOccurred())

			Expect(len(pods.Items)).To(Equal(1))
			Expect(pods.Items[0].Status.Phase).To(Equal(corev1.PodRunning))
		})

		It("Should have the performance CRD available in the cluster", func() {
			crd := &apiext.CustomResourceDefinition{}
			err := testclient.Client.Get(context.TODO(), goclient.ObjectKey{Name: utils.PerformanceCRDName}, crd)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("sriov", func() {
		It("should have the sriov namespace", func() {
			_, err := testclient.Client.Namespaces().Get(context.Background(), namespaces.SRIOVOperator, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should have the sriov operator deployment in running state", func() {
			deploy, err := testclient.Client.Deployments(namespaces.SRIOVOperator).Get(context.Background(), utils.SriovOperatorDeploymentName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(deploy.Status.Replicas).To(Equal(deploy.Status.ReadyReplicas))

			pods, err := testclient.Client.Pods(namespaces.SRIOVOperator).List(context.Background(), metav1.ListOptions{
				LabelSelector: fmt.Sprintf("name=%s", utils.SriovOperatorDeploymentName)})
			Expect(err).ToNot(HaveOccurred())

			Expect(len(pods.Items)).To(Equal(1))
			Expect(pods.Items[0].Status.Phase).To(Equal(corev1.PodRunning))
		})

		It("Should have the sriov CRDs available in the cluster", func() {
			crd := &apiext.CustomResourceDefinition{}
			err := testclient.Client.Get(context.TODO(), goclient.ObjectKey{Name: utils.SriovNetworkNodePolicies}, crd)
			Expect(err).ToNot(HaveOccurred())

			err = testclient.Client.Get(context.TODO(), goclient.ObjectKey{Name: utils.SriovNetworkNodeStates}, crd)
			Expect(err).ToNot(HaveOccurred())

			err = testclient.Client.Get(context.TODO(), goclient.ObjectKey{Name: utils.SriovNetworks}, crd)
			Expect(err).ToNot(HaveOccurred())

			err = testclient.Client.Get(context.TODO(), goclient.ObjectKey{Name: utils.SriovOperatorConfigs}, crd)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should deploy the injector pod if requested", func() {
			operatorConfig := &sriovv1.SriovOperatorConfig{}
			err := testclient.Client.Get(context.TODO(), goclient.ObjectKey{Name: "default", Namespace: namespaces.SRIOVOperator}, operatorConfig)
			Expect(err).ToNot(HaveOccurred())

			if *operatorConfig.Spec.EnableInjector {
				daemonset, err := testclient.Client.DaemonSets(namespaces.SRIOVOperator).Get(context.Background(), "network-resources-injector", metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(daemonset.Status.DesiredNumberScheduled).To(Equal(daemonset.Status.NumberReady))
			} else {
				_, err := testclient.Client.DaemonSets(namespaces.SRIOVOperator).Get(context.Background(), "network-resources-injector", metav1.GetOptions{})
				Expect(err).To(HaveOccurred())
				Expect(errors.IsNotFound(err)).To(BeTrue())
			}
		})

		It("should deploy the operator webhook if requested", func() {
			operatorConfig := &sriovv1.SriovOperatorConfig{}
			err := testclient.Client.Get(context.TODO(), goclient.ObjectKey{Name: "default", Namespace: namespaces.SRIOVOperator}, operatorConfig)
			Expect(err).ToNot(HaveOccurred())

			if *operatorConfig.Spec.EnableOperatorWebhook {
				daemonset, err := testclient.Client.DaemonSets(namespaces.SRIOVOperator).Get(context.Background(), "operator-webhook", metav1.GetOptions{})
				Expect(err).ToNot(HaveOccurred())
				Expect(daemonset.Status.DesiredNumberScheduled).To(Equal(daemonset.Status.NumberReady))
			} else {
				_, err := testclient.Client.DaemonSets(namespaces.SRIOVOperator).Get(context.Background(), "operator-webhook", metav1.GetOptions{})
				Expect(err).To(HaveOccurred())
				Expect(errors.IsNotFound(err)).To(BeTrue())
			}
		})

		It("should have SR-IOV node statuses not in progress", func() {
			sriovclient := sriovtestclient.New("")
			sriov.WaitStable(sriovclient)
		})
	})

	Context("sctp", func() {
		matchSCTPMachineConfig := func(ignitionConfig *igntypes.Config) bool {
			if ignitionConfig.Storage.Files != nil {
				for _, file := range ignitionConfig.Storage.Files {
					if file.Path == "/etc/modprobe.d/sctp-blacklist.conf" {
						return true
					}
				}
			}
			return false
		}

		It("should have a sctp enable machine config", func() {
			exist, _ := findMatchingMachineConfig(matchSCTPMachineConfig)
			Expect(exist).To(BeTrue(), "was not able to find a sctp machine config")
		})

		It("should have the sctp enable machine config as part of the CNF machine config pool", func() {
			mcpExist, _ := findMachineConfigPoolForMC(matchSCTPMachineConfig)
			Expect(mcpExist).To(BeTrue(), "was not able to find the sctp machine config in a machine config pool")
		})
	})

	Context("ptp", func() {
		It("should have the ptp namespace", func() {
			_, err := testclient.Client.Namespaces().Get(context.Background(), utils.PtpNamespace, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should have the ptp operator deployment in running state", func() {
			deploy, err := testclient.Client.Deployments(utils.PtpNamespace).Get(context.Background(), utils.PtpOperatorDeploymentName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(deploy.Status.Replicas).To(Equal(deploy.Status.ReadyReplicas))

			pods, err := testclient.Client.Pods(utils.PtpNamespace).List(context.Background(), metav1.ListOptions{
				LabelSelector: fmt.Sprintf("name=%s", utils.PtpOperatorDeploymentName)})
			Expect(err).ToNot(HaveOccurred())

			Expect(len(pods.Items)).To(Equal(1))
			Expect(pods.Items[0].Status.Phase).To(Equal(corev1.PodRunning))
		})

		It("should have the linuxptp daemonset in running state", func() {
			daemonset, err := testclient.Client.DaemonSets(utils.PtpNamespace).Get(context.Background(), utils.PtpDaemonsetName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(daemonset.Status.NumberReady).To(Equal(daemonset.Status.DesiredNumberScheduled))
		})

		It("should have the ptp CRDs available in the cluster", func() {
			crd := &apiext.CustomResourceDefinition{}
			err := testclient.Client.Get(context.TODO(), goclient.ObjectKey{Name: utils.NodePtpDevices}, crd)
			Expect(err).ToNot(HaveOccurred())

			err = testclient.Client.Get(context.TODO(), goclient.ObjectKey{Name: utils.PtpConfigs}, crd)
			Expect(err).ToNot(HaveOccurred())

			err = testclient.Client.Get(context.TODO(), goclient.ObjectKey{Name: utils.PtpOperatorConfigs}, crd)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("dpdk", func() {
		It("should have a tag ready from the dpdk imagestream", func() {
			imagestream, err := testclient.Client.ImageStreams("dpdk").Get(context.TODO(), "s2i-dpdk-app", metav1.GetOptions{})
			if errors.IsNotFound(err) {
				Skip("No dpdk imagestream found, relying on dpdk image")
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(len(imagestream.Status.Tags)).To(BeNumerically(">", 0), "dpdk imagestream has no tags")
		})
	})

	Context("xt_u32", func() {
		matchXT_U32MachineConfig := func(ignitionConfig *igntypes.Config) bool {
			if ignitionConfig.Storage.Files != nil {
				for _, file := range ignitionConfig.Storage.Files {
					if file.Path == "/etc/modules-load.d/xt_u32-load.conf" {
						return true
					}
				}
			}
			return false
		}

		It("should have a xt_u32 enable machine config", func() {
			exist, _ := findMatchingMachineConfig(matchXT_U32MachineConfig)
			Expect(exist).To(BeTrue(), "was not able to find a xt_u32 machine config")
		})

		It("should have the xt_u32 enable machine config as part of the CNF machine config pool", func() {
			mcpExist, _ := findMachineConfigPoolForMC(matchXT_U32MachineConfig)
			Expect(mcpExist).To(BeTrue(), "was not able to find the xt_u32 machine config in a machine config pool")
		})
	})
})

type MCMatcher func(*igntypes.Config) bool

func findMatchingMachineConfig(match MCMatcher) (bool, *clientmachineconfigv1.MachineConfig) {
	mcl := &clientmachineconfigv1.MachineConfigList{}
	err := testclient.Client.List(context.TODO(), mcl)
	Expect(err).ToNot(HaveOccurred())
	for _, mc := range mcl.Items {
		if mc.Spec.Config.Raw == nil {
			continue
		}
		ignitionConfig := igntypes.Config{}
		err := json.Unmarshal(mc.Spec.Config.Raw, &ignitionConfig)
		Expect(err).ToNot(HaveOccurred(), "Failed to unmarshal raw config for ", mc.Name)

		if match(&ignitionConfig) {
			return true, &mc
		}
	}
	return false, nil
}

func findMachineConfigPool(label, name string) (bool, *clientmachineconfigv1.MachineConfigPool) {
	mcp := &clientmachineconfigv1.MachineConfigPoolList{}
	err := testclient.Client.List(context.TODO(), mcp)
	Expect(err).ToNot(HaveOccurred())

	for _, mcpItem := range mcp.Items {
		if mcpItem.Spec.MachineConfigSelector != nil {
			if mcpItem.Spec.MachineConfigSelector.MatchExpressions != nil {
				for _, expression := range mcpItem.Spec.MachineConfigSelector.MatchExpressions {
					if expression.Key == label {
						for _, value := range expression.Values {
							if value == name {
								return true, &mcpItem
							}
						}
					}
				}
			}
			if mcpItem.Spec.MachineConfigSelector.MatchLabels != nil {
				for _, value := range mcpItem.Spec.MachineConfigSelector.MatchLabels {
					if value == name {
						return true, &mcpItem
					}
				}
			}
		}
	}

	return false, nil
}

func findMachineConfigPoolForMC(match MCMatcher) (bool, *clientmachineconfigv1.MachineConfigPool) {
	exist, mc := findMatchingMachineConfig(match)
	Expect(exist).To(BeTrue())
	machineConfigRole := "machineconfiguration.openshift.io/role"
	mcpExist, mcp := findMachineConfigPool(machineConfigRole, mc.Labels[machineConfigRole])
	Expect(mcpExist).To(BeTrue(), fmt.Sprintf("was not able to find a machine config pool with the machine config selector of %s=%s", machineConfigRole, mc.Labels[machineConfigRole]))

	mcpExist = false
	for _, configuration := range mcp.Status.Configuration.Source {
		if configuration.Name == mc.Name {
			mcpExist = true
			break
		}
	}

	Expect(mcpExist).To(BeTrue(), fmt.Sprintf("was not able to find the machine config %s in the %s machine config pool", mc.Name, mcp.Name))
	return mcpExist, mcp
}
