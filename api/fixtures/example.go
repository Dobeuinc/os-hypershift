package fixtures

import (
	"crypto/rand"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"

	hyperv1 "github.com/openshift/hypershift/api/v1alpha1"

	configv1 "github.com/openshift/api/config/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type ExampleResources struct {
	AdditionalTrustBundle *corev1.ConfigMap
	Namespace             *corev1.Namespace
	PullSecret            *corev1.Secret
	Resources             []crclient.Object
	SSHKey                *corev1.Secret
	Cluster               *hyperv1.HostedCluster
	NodePools             []*hyperv1.NodePool
}

func (o *ExampleResources) AsObjects() []crclient.Object {
	objects := []crclient.Object{
		o.Namespace,
		o.PullSecret,
		o.Cluster,
	}
	objects = append(objects, o.Resources...)
	if o.SSHKey != nil {
		objects = append(objects, o.SSHKey)
	}
	if o.AdditionalTrustBundle != nil {
		objects = append(objects, o.AdditionalTrustBundle)
	}
	for _, nodePool := range o.NodePools {
		objects = append(objects, nodePool)
	}
	return objects
}

type ExampleOptions struct {
	AdditionalTrustBundle            string
	Namespace                        string
	Name                             string
	ReleaseImage                     string
	PullSecret                       []byte
	IssuerURL                        string
	SSHPublicKey                     []byte
	SSHPrivateKey                    []byte
	NodePoolReplicas                 int32
	ImageContentSources              []hyperv1.ImageContentSource
	InfraID                          string
	ComputeCIDR                      string
	ServiceCIDR                      string
	PodCIDR                          string
	BaseDomain                       string
	PublicZoneID                     string
	PrivateZoneID                    string
	Annotations                      map[string]string
	FIPS                             bool
	AutoRepair                       bool
	EtcdStorageClass                 string
	ExternalDNSDomain                string
	AWS                              *ExampleAWSOptions
	None                             *ExampleNoneOptions
	Agent                            *ExampleAgentOptions
	Kubevirt                         *ExampleKubevirtOptions
	Azure                            *ExampleAzureOptions
	PowerVS                          *ExamplePowerVSOptions
	NetworkType                      hyperv1.NetworkType
	ControlPlaneAvailabilityPolicy   hyperv1.AvailabilityPolicy
	InfrastructureAvailabilityPolicy hyperv1.AvailabilityPolicy
}

type ExampleNoneOptions struct {
	APIServerAddress string
}

type ExampleAgentOptions struct {
	APIServerAddress string
	AgentNamespace   string
}

type ExampleKubevirtOptions struct {
	ServicePublishingStrategy string
	APIServerAddress          string
	Memory                    string
	Cores                     uint32
	Image                     string
	RootVolumeSize            uint32
	RootVolumeStorageClass    string
}

type ExampleAWSOptionsZones struct {
	Name     string
	SubnetID *string
}

type ExampleAWSOptions struct {
	Region                      string
	Zones                       []ExampleAWSOptionsZones
	VPCID                       string
	SecurityGroupID             string
	InstanceProfile             string
	InstanceType                string
	Roles                       []hyperv1.AWSRoleCredentials
	KubeCloudControllerRoleARN  string
	NodePoolManagementRoleARN   string
	ControlPlaneOperatorRoleARN string
	KMSProviderRoleARN          string
	KMSKeyARN                   string
	RootVolumeSize              int64
	RootVolumeType              string
	RootVolumeIOPS              int64
	ResourceTags                []hyperv1.AWSResourceTag
	EndpointAccess              string
	ProxyAddress                string
}

type ExampleAzureOptions struct {
	Creds             AzureCreds
	Location          string
	ResourceGroupName string
	VnetName          string
	VnetID            string
	SubnetName        string
	BootImageID       string
	MachineIdentityID string
	InstanceType      string
	SecurityGroupName string
	DiskSizeGB        int32
	AvailabilityZones []string
}

// AzureCreds is the fileformat we expect for credentials. It is copied from the installer
// to allow using the same crededentials file for both:
// https://github.com/openshift/installer/blob/8fca1ade5b096d9b2cd312c4599881d099439288/pkg/asset/installconfig/azure/session.go#L36
type AzureCreds struct {
	SubscriptionID string `json:"subscriptionId,omitempty"`
	ClientID       string `json:"clientId,omitempty"`
	ClientSecret   string `json:"clientSecret,omitempty"`
	TenantID       string `json:"tenantId,omitempty"`
}

func (o ExampleOptions) Resources() *ExampleResources {
	namespace := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: o.Namespace,
		},
	}

	pullSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace.Name,
			Name:      o.Name + "-pull-secret",
		},
		Data: map[string][]byte{
			".dockerconfigjson": o.PullSecret,
		},
	}

	var sshKeySecret *corev1.Secret
	var sshKeyReference corev1.LocalObjectReference
	if len(o.SSHPublicKey) > 0 {
		sshKeySecret = &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: corev1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace.Name,
				Name:      o.Name + "-ssh-key",
			},
			Data: map[string][]byte{
				"id_rsa.pub": o.SSHPublicKey,
			},
		}
		if len(o.SSHPrivateKey) > 0 {
			sshKeySecret.Data["id_rsa"] = o.SSHPrivateKey
		}
		sshKeyReference = corev1.LocalObjectReference{Name: sshKeySecret.Name}
	}

	var platformSpec hyperv1.PlatformSpec
	var resources []crclient.Object
	var services []hyperv1.ServicePublishingStrategyMapping
	var secretEncryption *hyperv1.SecretEncryptionSpec
	var globalOpts []runtime.RawExtension

	switch {
	case o.AWS != nil:
		buildAWSCreds := func(name, arn string) *corev1.Secret {
			return &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: corev1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace.Name,
					Name:      name,
				},
				Data: map[string][]byte{
					"credentials": []byte(fmt.Sprintf(`[default]
role_arn = %s
web_identity_token_file = /var/run/secrets/openshift/serviceaccount/token
`, arn)),
				},
			}
		}

		var kmsCredsSecret *corev1.Secret
		if len(o.AWS.KMSProviderRoleARN) > 0 {
			kmsCredsSecret = buildAWSCreds(o.Name+"-kms-creds", o.AWS.KMSProviderRoleARN)
		}
		awsResources := &ExampleAWSResources{
			buildAWSCreds(o.Name+"-cloud-ctrl-creds", o.AWS.KubeCloudControllerRoleARN),
			buildAWSCreds(o.Name+"-node-mgmt-creds", o.AWS.NodePoolManagementRoleARN),
			buildAWSCreds(o.Name+"-cpo-creds", o.AWS.ControlPlaneOperatorRoleARN),
			kmsCredsSecret,
		}
		endpointAccess := hyperv1.AWSEndpointAccessType(o.AWS.EndpointAccess)
		resources = awsResources.AsObjects()
		platformSpec = hyperv1.PlatformSpec{
			Type: hyperv1.AWSPlatform,
			AWS: &hyperv1.AWSPlatformSpec{
				Region: o.AWS.Region,
				Roles:  o.AWS.Roles,
				CloudProviderConfig: &hyperv1.AWSCloudProviderConfig{
					VPC: o.AWS.VPCID,
					Subnet: &hyperv1.AWSResourceReference{
						ID: o.AWS.Zones[0].SubnetID,
					},
					Zone: o.AWS.Zones[0].Name,
				},
				KubeCloudControllerCreds:  corev1.LocalObjectReference{Name: awsResources.KubeCloudControllerAWSCreds.Name},
				NodePoolManagementCreds:   corev1.LocalObjectReference{Name: awsResources.NodePoolManagementAWSCreds.Name},
				ControlPlaneOperatorCreds: corev1.LocalObjectReference{Name: awsResources.ControlPlaneOperatorAWSCreds.Name},
				ResourceTags:              o.AWS.ResourceTags,
				EndpointAccess:            endpointAccess,
			},
		}

		if o.AWS.ProxyAddress != "" {
			globalOpts = append(globalOpts, runtime.RawExtension{Object: &configv1.Proxy{
				TypeMeta: metav1.TypeMeta{
					APIVersion: configv1.GroupVersion.String(),
					Kind:       "Proxy",
				},
				Spec: configv1.ProxySpec{
					HTTPProxy:  o.AWS.ProxyAddress,
					HTTPSProxy: o.AWS.ProxyAddress,
				},
			}})
		}

		if kmsCredsSecret != nil {
			secretEncryption = &hyperv1.SecretEncryptionSpec{
				Type: hyperv1.KMS,
				KMS: &hyperv1.KMSSpec{
					Provider: hyperv1.AWS,
					AWS: &hyperv1.AWSKMSSpec{
						Region: o.AWS.Region,
						ActiveKey: hyperv1.AWSKMSKeyEntry{
							ARN: o.AWS.KMSKeyARN,
						},
						Auth: hyperv1.AWSKMSAuthSpec{
							Credentials: corev1.LocalObjectReference{
								Name: awsResources.KMSProviderAWSCreds.Name,
							},
						},
					},
				},
			}
		}
		services = getIngressServicePublishingStrategyMapping(o.NetworkType)
		if o.ExternalDNSDomain != "" {
			for i, svc := range services {
				switch svc.Service {
				case hyperv1.APIServer:
					if endpointAccess != hyperv1.Private {
						services[i].LoadBalancer = &hyperv1.LoadBalancerPublishingStrategy{
							Hostname: fmt.Sprintf("api-%s.%s", o.Name, o.ExternalDNSDomain),
						}
					}

				case hyperv1.OAuthServer:
					if endpointAccess != hyperv1.Private {
						services[i].Route = &hyperv1.RoutePublishingStrategy{
							Hostname: fmt.Sprintf("oauth-%s.%s", o.Name, o.ExternalDNSDomain),
						}
					}

				case hyperv1.Konnectivity:
					if endpointAccess == hyperv1.Public {
						services[i].Route = &hyperv1.RoutePublishingStrategy{
							Hostname: fmt.Sprintf("konnectivity-%s.%s", o.Name, o.ExternalDNSDomain),
						}
					}

				case hyperv1.Ignition:
					if endpointAccess == hyperv1.Public {
						services[i].Route = &hyperv1.RoutePublishingStrategy{
							Hostname: fmt.Sprintf("ignition-%s.%s", o.Name, o.ExternalDNSDomain),
						}
					}
				case hyperv1.OVNSbDb:
					if endpointAccess == hyperv1.Public {
						services[i].Route = &hyperv1.RoutePublishingStrategy{
							Hostname: fmt.Sprintf("ovn-sbdb-%s.%s", o.Name, o.ExternalDNSDomain),
						}
					}
				}
			}

		}
	case o.None != nil:
		platformSpec = hyperv1.PlatformSpec{
			Type: hyperv1.NonePlatform,
		}
		services = getServicePublishingStrategyMappingByAPIServerAddress(o.None.APIServerAddress, o.NetworkType)
	case o.Agent != nil:
		platformSpec = hyperv1.PlatformSpec{
			Type: hyperv1.AgentPlatform,
			Agent: &hyperv1.AgentPlatformSpec{
				AgentNamespace: o.Agent.AgentNamespace,
			},
		}
		agentResources := &ExampleAgentResources{
			&rbacv1.Role{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Role",
					APIVersion: rbacv1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: o.Agent.AgentNamespace,
					Name:      "capi-provider-role",
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{"agent-install.openshift.io"},
						Resources: []string{"agents"},
						Verbs:     []string{"*"},
					},
				},
			},
		}
		resources = agentResources.AsObjects()
		services = getServicePublishingStrategyMappingByAPIServerAddress(o.Agent.APIServerAddress, o.NetworkType)
	case o.Kubevirt != nil:
		platformSpec = hyperv1.PlatformSpec{
			Type: hyperv1.KubevirtPlatform,
		}
		switch o.Kubevirt.ServicePublishingStrategy {
		case "NodePort":
			services = getServicePublishingStrategyMappingByAPIServerAddress(o.Kubevirt.APIServerAddress, o.NetworkType)
		case "Ingress":
			services = getIngressServicePublishingStrategyMapping(o.NetworkType)
		default:
			panic(fmt.Sprintf("service publishing type %s is not supported", o.Kubevirt.ServicePublishingStrategy))
		}
	case o.Azure != nil:
		credentialSecret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      o.Name + "-cloud-credentials",
				Namespace: namespace.Name,
			},
			Data: map[string][]byte{
				"AZURE_SUBSCRIPTION_ID": []byte(o.Azure.Creds.SubscriptionID),
				"AZURE_TENANT_ID":       []byte(o.Azure.Creds.TenantID),
				"AZURE_CLIENT_ID":       []byte(o.Azure.Creds.ClientID),
				"AZURE_CLIENT_SECRET":   []byte(o.Azure.Creds.ClientSecret),
			},
		}
		resources = append(resources, credentialSecret)

		platformSpec = hyperv1.PlatformSpec{
			Type: hyperv1.AzurePlatform,
			Azure: &hyperv1.AzurePlatformSpec{
				Credentials:       corev1.LocalObjectReference{Name: credentialSecret.Name},
				Location:          o.Azure.Location,
				ResourceGroupName: o.Azure.ResourceGroupName,
				VnetName:          o.Azure.VnetName,
				VnetID:            o.Azure.VnetID,
				SubnetName:        o.Azure.SubnetName,
				SubscriptionID:    o.Azure.Creds.SubscriptionID,
				MachineIdentityID: o.Azure.MachineIdentityID,
				SecurityGroupName: o.Azure.SecurityGroupName,
			},
		}
		services = getIngressServicePublishingStrategyMapping(o.NetworkType)

	case o.PowerVS != nil:
		buildIBMCloudCreds := func(name, apikey string) *corev1.Secret {
			data := map[string][]byte{
				"ibm-credentials.env": []byte(fmt.Sprintf(`IBMCLOUD_AUTH_TYPE=iam
 IBMCLOUD_APIKEY=%s
 IBMCLOUD_AUTH_URL=https://iam.cloud.ibm.com
 `, apikey)),
				"ibmcloud_api_key": []byte(apikey),
			}

			return &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: corev1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace.Name,
					Name:      name,
				},
				Data: data,
			}
		}

		// TODO(dharaneeshvrd): Need exploration to use granular permissions
		powerVSResources := &ExamplePowerVSResources{
			buildIBMCloudCreds(o.Name+"-cloud-ctrl-creds", o.PowerVS.ApiKey),
			buildIBMCloudCreds(o.Name+"-node-mgmt-creds", o.PowerVS.ApiKey),
			buildIBMCloudCreds(o.Name+"-cpo-creds", o.PowerVS.ApiKey),
			buildIBMCloudCreds(o.Name+"-ingress-creds", o.PowerVS.ApiKey),
		}
		resources = powerVSResources.AsObjects()
		platformSpec = hyperv1.PlatformSpec{
			Type: hyperv1.PowerVSPlatform,
			PowerVS: &hyperv1.PowerVSPlatformSpec{
				AccountID:         o.PowerVS.AccountID,
				ResourceGroup:     o.PowerVS.ResourceGroup,
				Region:            o.PowerVS.Region,
				Zone:              o.PowerVS.Zone,
				CISInstanceCRN:    o.PowerVS.CISInstanceCRN,
				ServiceInstanceID: o.PowerVS.CloudInstanceID,
				Subnet: &hyperv1.PowerVSResourceReference{
					Name: &o.PowerVS.Subnet,
					ID:   &o.PowerVS.SubnetID,
				},
				VPC: &hyperv1.PowerVSVPC{
					Name:   o.PowerVS.Vpc,
					Region: o.PowerVS.VpcRegion,
					Subnet: o.PowerVS.VpcSubnet,
				},
				KubeCloudControllerCreds:  corev1.LocalObjectReference{Name: powerVSResources.KubeCloudControllerCreds.Name},
				NodePoolManagementCreds:   corev1.LocalObjectReference{Name: powerVSResources.NodePoolManagementCreds.Name},
				ControlPlaneOperatorCreds: corev1.LocalObjectReference{Name: powerVSResources.ControlPlaneOperatorCreds.Name},
				IngressOperatorCloudCreds: corev1.LocalObjectReference{Name: powerVSResources.IngressOperatorCloudCreds.Name},
			},
		}
		services = getIngressServicePublishingStrategyMapping(o.NetworkType)
	default:
		panic("no platform specified")
	}

	// If secret encryption was not specified, default to AESCBC
	if secretEncryption == nil {
		encryptionSecret := o.EtcdEncryptionKeySecret()
		resources = append(resources, encryptionSecret)
		secretEncryption = &hyperv1.SecretEncryptionSpec{
			Type: hyperv1.AESCBC,
			AESCBC: &hyperv1.AESCBCSpec{
				ActiveKey: corev1.LocalObjectReference{
					Name: encryptionSecret.Name,
				},
			},
		}
	}

	var etcdStorgageClass *string = nil
	if len(o.EtcdStorageClass) > 0 {
		etcdStorgageClass = pointer.StringPtr(o.EtcdStorageClass)
	}
	cluster := &hyperv1.HostedCluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HostedCluster",
			APIVersion: hyperv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace.Name,
			Name:        o.Name,
			Annotations: o.Annotations,
		},
		Spec: hyperv1.HostedClusterSpec{
			Release: hyperv1.Release{
				Image: o.ReleaseImage,
			},
			Etcd: hyperv1.EtcdSpec{
				ManagementType: hyperv1.Managed,
				Managed: &hyperv1.ManagedEtcdSpec{
					Storage: hyperv1.ManagedEtcdStorageSpec{
						Type: hyperv1.PersistentVolumeEtcdStorage,
						PersistentVolume: &hyperv1.PersistentVolumeEtcdStorageSpec{
							StorageClassName: etcdStorgageClass,
							Size:             &hyperv1.DefaultPersistentVolumeEtcdStorageSize,
						},
					},
				},
			},
			SecretEncryption: secretEncryption,
			Networking: hyperv1.ClusterNetworking{
				ServiceCIDR: o.ServiceCIDR,
				PodCIDR:     o.PodCIDR,
				MachineCIDR: o.ComputeCIDR,
				NetworkType: o.NetworkType,
			},
			Services:   services,
			InfraID:    o.InfraID,
			PullSecret: corev1.LocalObjectReference{Name: pullSecret.Name},
			IssuerURL:  o.IssuerURL,
			SSHKey:     sshKeyReference,
			FIPS:       o.FIPS,
			DNS: hyperv1.DNSSpec{
				BaseDomain:    o.BaseDomain,
				PublicZoneID:  o.PublicZoneID,
				PrivateZoneID: o.PrivateZoneID,
			},
			ControllerAvailabilityPolicy:     o.ControlPlaneAvailabilityPolicy,
			InfrastructureAvailabilityPolicy: o.InfrastructureAvailabilityPolicy,
			Platform:                         platformSpec,
		},
	}

	if len(globalOpts) > 0 {
		cluster.Spec.Configuration = &hyperv1.ClusterConfiguration{Items: globalOpts}
	}

	var userCABundleCM *corev1.ConfigMap
	if len(o.AdditionalTrustBundle) > 0 {
		userCABundleCM = &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: corev1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "user-ca-bundle",
				Namespace: namespace.Name,
			},
			Data: map[string]string{
				"ca-bundle.crt": string(o.AdditionalTrustBundle),
			},
		}
		cluster.Spec.AdditionalTrustBundle = &corev1.LocalObjectReference{Name: userCABundleCM.Name}
	}

	if len(o.ImageContentSources) > 0 {
		cluster.Spec.ImageContentSources = o.ImageContentSources
	}

	if o.NodePoolReplicas <= -1 {
		return &ExampleResources{
			AdditionalTrustBundle: userCABundleCM,
			Namespace:             namespace,
			PullSecret:            pullSecret,
			Resources:             resources,
			SSHKey:                sshKeySecret,
			Cluster:               cluster,
			NodePools:             []*hyperv1.NodePool{},
		}
	}

	defaultNodePool := func(name string) *hyperv1.NodePool {
		return &hyperv1.NodePool{
			TypeMeta: metav1.TypeMeta{
				Kind:       "NodePool",
				APIVersion: hyperv1.GroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace.Name,
				Name:      name,
			},
			Spec: hyperv1.NodePoolSpec{
				Management: hyperv1.NodePoolManagement{
					AutoRepair:  o.AutoRepair,
					UpgradeType: hyperv1.UpgradeTypeReplace,
				},
				Replicas:    &o.NodePoolReplicas,
				ClusterName: o.Name,
				Release: hyperv1.Release{
					Image: o.ReleaseImage,
				},
				Platform: hyperv1.NodePoolPlatform{
					Type: cluster.Spec.Platform.Type,
				},
			},
		}
	}

	var nodePools []*hyperv1.NodePool
	switch cluster.Spec.Platform.Type {
	case hyperv1.AWSPlatform:
		for _, zone := range o.AWS.Zones {
			nodePool := defaultNodePool(fmt.Sprintf("%s-%s", cluster.Name, zone.Name))
			nodePool.Spec.Platform.AWS = &hyperv1.AWSNodePoolPlatform{
				InstanceType:    o.AWS.InstanceType,
				InstanceProfile: o.AWS.InstanceProfile,
				Subnet: &hyperv1.AWSResourceReference{
					ID: zone.SubnetID,
				},
				SecurityGroups: []hyperv1.AWSResourceReference{
					{
						ID: &o.AWS.SecurityGroupID,
					},
				},
				RootVolume: &hyperv1.Volume{
					Size: o.AWS.RootVolumeSize,
					Type: o.AWS.RootVolumeType,
					IOPS: o.AWS.RootVolumeIOPS,
				},
			}
			nodePools = append(nodePools, nodePool)
		}
	case hyperv1.KubevirtPlatform:
		nodePool := defaultNodePool(cluster.Name)
		nodePool.Spec.Platform.Kubevirt = ExampleKubeVirtTemplate(o.Kubevirt)
		nodePools = append(nodePools, nodePool)
	case hyperv1.NonePlatform, hyperv1.AgentPlatform:
		nodePools = append(nodePools, defaultNodePool(cluster.Name))
	case hyperv1.AzurePlatform:
		if len(o.Azure.AvailabilityZones) > 0 {
			for _, availabilityZone := range o.Azure.AvailabilityZones {
				nodePool := defaultNodePool(fmt.Sprintf("%s-%s", cluster.Name, availabilityZone))
				nodePool.Spec.Platform.Azure = &hyperv1.AzureNodePoolPlatform{
					VMSize:           o.Azure.InstanceType,
					ImageID:          o.Azure.BootImageID,
					DiskSizeGB:       o.Azure.DiskSizeGB,
					AvailabilityZone: availabilityZone,
				}
				nodePools = append(nodePools, nodePool)
			}

		} else {
			nodePool := defaultNodePool(cluster.Name)
			nodePool.Spec.Platform.Azure = &hyperv1.AzureNodePoolPlatform{
				VMSize:     o.Azure.InstanceType,
				ImageID:    o.Azure.BootImageID,
				DiskSizeGB: o.Azure.DiskSizeGB,
			}
			nodePools = append(nodePools, nodePool)
		}
	case hyperv1.PowerVSPlatform:
		nodePool := defaultNodePool(cluster.Name)
		nodePool.Spec.Platform.PowerVS = &hyperv1.PowerVSNodePoolPlatform{
			SystemType:    o.PowerVS.SysType,
			ProcessorType: o.PowerVS.ProcType,
			Processors:    intstr.FromString(o.PowerVS.Processors),
			MemoryGiB:     o.PowerVS.Memory,
		}
		nodePools = append(nodePools, nodePool)
	default:
		panic("Unsupported platform")
	}

	return &ExampleResources{
		AdditionalTrustBundle: userCABundleCM,
		Namespace:             namespace,
		PullSecret:            pullSecret,
		Resources:             resources,
		SSHKey:                sshKeySecret,
		Cluster:               cluster,
		NodePools:             nodePools,
	}
}

func getIngressServicePublishingStrategyMapping(netType hyperv1.NetworkType) []hyperv1.ServicePublishingStrategyMapping {

	ret := []hyperv1.ServicePublishingStrategyMapping{
		{
			Service: hyperv1.APIServer,
			ServicePublishingStrategy: hyperv1.ServicePublishingStrategy{
				Type: hyperv1.LoadBalancer,
			},
		},
		{
			Service: hyperv1.OAuthServer,
			ServicePublishingStrategy: hyperv1.ServicePublishingStrategy{
				Type: hyperv1.Route,
			},
		},
		{
			Service: hyperv1.Konnectivity,
			ServicePublishingStrategy: hyperv1.ServicePublishingStrategy{
				Type: hyperv1.Route,
			},
		},
		{
			Service: hyperv1.Ignition,
			ServicePublishingStrategy: hyperv1.ServicePublishingStrategy{
				Type: hyperv1.Route,
			},
		},
	}
	if netType == hyperv1.OVNKubernetes {
		ret = append(ret, hyperv1.ServicePublishingStrategyMapping{
			Service: hyperv1.OVNSbDb,
			ServicePublishingStrategy: hyperv1.ServicePublishingStrategy{
				Type: hyperv1.Route,
			},
		})
	}
	return ret
}

func getIngressWithHostnameServicePublishingStrategyMapping(name, domain string, endpointAcesss hyperv1.AWSEndpointAccessType, netType hyperv1.NetworkType) []hyperv1.ServicePublishingStrategyMapping {
	ret := []hyperv1.ServicePublishingStrategyMapping{
		{
			Service: hyperv1.APIServer,
			ServicePublishingStrategy: hyperv1.ServicePublishingStrategy{
				Type: hyperv1.LoadBalancer,
				LoadBalancer: &hyperv1.LoadBalancerPublishingStrategy{
					Hostname: fmt.Sprintf("api-%s.%s", name, domain),
				},
			},
		},
		{
			Service: hyperv1.OAuthServer,
			ServicePublishingStrategy: hyperv1.ServicePublishingStrategy{
				Type: hyperv1.Route,
				Route: &hyperv1.RoutePublishingStrategy{
					Hostname: fmt.Sprintf("oauth-%s.%s", name, domain),
				},
			},
		},
		{
			Service: hyperv1.Konnectivity,
			ServicePublishingStrategy: hyperv1.ServicePublishingStrategy{
				Type: hyperv1.Route,
				Route: &hyperv1.RoutePublishingStrategy{
					Hostname: fmt.Sprintf("konnectivity-%s.%s", name, domain),
				},
			},
		},
		{
			Service: hyperv1.Ignition,
			ServicePublishingStrategy: hyperv1.ServicePublishingStrategy{
				Type: hyperv1.Route,
				Route: &hyperv1.RoutePublishingStrategy{
					Hostname: fmt.Sprintf("ignition-%s.%s", name, domain),
				},
			},
		},
	}
	if netType == hyperv1.OVNKubernetes {
		ret = append(ret, hyperv1.ServicePublishingStrategyMapping{
			Service: hyperv1.OVNSbDb,
			ServicePublishingStrategy: hyperv1.ServicePublishingStrategy{
				Type: hyperv1.Route,
			},
		})
	}
	return ret
}

func getServicePublishingStrategyMappingByAPIServerAddress(APIServerAddress string, netType hyperv1.NetworkType) []hyperv1.ServicePublishingStrategyMapping {
	ret := []hyperv1.ServicePublishingStrategyMapping{
		{
			Service: hyperv1.APIServer,
			ServicePublishingStrategy: hyperv1.ServicePublishingStrategy{
				Type:     hyperv1.NodePort,
				NodePort: &hyperv1.NodePortPublishingStrategy{Address: APIServerAddress},
			},
		},
		{
			Service: hyperv1.OAuthServer,
			ServicePublishingStrategy: hyperv1.ServicePublishingStrategy{
				Type:     hyperv1.NodePort,
				NodePort: &hyperv1.NodePortPublishingStrategy{Address: APIServerAddress},
			},
		},
		{
			Service: hyperv1.OIDC,
			ServicePublishingStrategy: hyperv1.ServicePublishingStrategy{
				Type:     hyperv1.None,
				NodePort: &hyperv1.NodePortPublishingStrategy{Address: APIServerAddress},
			},
		},
		{
			Service: hyperv1.Konnectivity,
			ServicePublishingStrategy: hyperv1.ServicePublishingStrategy{
				Type:     hyperv1.NodePort,
				NodePort: &hyperv1.NodePortPublishingStrategy{Address: APIServerAddress},
			},
		},
		{
			Service: hyperv1.Ignition,
			ServicePublishingStrategy: hyperv1.ServicePublishingStrategy{
				Type:     hyperv1.NodePort,
				NodePort: &hyperv1.NodePortPublishingStrategy{Address: APIServerAddress},
			},
		},
	}
	if netType == hyperv1.OVNKubernetes {
		ret = append(ret, hyperv1.ServicePublishingStrategyMapping{
			Service: hyperv1.OVNSbDb,
			ServicePublishingStrategy: hyperv1.ServicePublishingStrategy{
				Type:     hyperv1.NodePort,
				NodePort: &hyperv1.NodePortPublishingStrategy{Address: APIServerAddress},
			},
		})
	}
	return ret
}

func (o ExampleOptions) EtcdEncryptionKeySecret() *corev1.Secret {
	generatedKey := make([]byte, 32)
	_, err := rand.Read(generatedKey)
	if err != nil {
		panic(fmt.Sprintf("failed to generate random etcd key: %v", err))
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.Name + "-etcd-encryption-key",
			Namespace: o.Namespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		Data: map[string][]byte{
			hyperv1.AESCBCKeySecretKey: generatedKey,
		},
		Type: corev1.SecretTypeOpaque,
	}
}
