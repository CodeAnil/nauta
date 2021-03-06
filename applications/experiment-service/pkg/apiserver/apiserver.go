/*
Copyright 2018 Intel Corporation.
Copyright The Kubernetes Authors.

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

package apiserver

import (
	"k8s.io/apimachinery/pkg/apimachinery/announced"
	"k8s.io/apimachinery/pkg/apimachinery/registered"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"

	"github.com/nervanasystems/carbon/applications/experiment-service/pkg/apis/aggregator"
	"github.com/nervanasystems/carbon/applications/experiment-service/pkg/apis/aggregator/install"
	"github.com/nervanasystems/carbon/applications/experiment-service/pkg/apis/aggregator/v1"
	informers "github.com/nervanasystems/carbon/applications/experiment-service/pkg/client/informers/internalversion"
	aggregatorregistry "github.com/nervanasystems/carbon/applications/experiment-service/pkg/registry"
	runStorage "github.com/nervanasystems/carbon/applications/experiment-service/pkg/registry/aggregator/run"
)

var (
	groupFactoryRegistry = make(announced.APIGroupFactoryRegistry)
	registry             = registered.NewOrDie("")
	Scheme               = runtime.NewScheme()
	Codecs               = serializer.NewCodecFactory(Scheme)
)

func init() {
	install.Install(groupFactoryRegistry, registry, Scheme)

	// we need to add the options to empty v1
	// TODO fix the server code to avoid this
	metav1.AddToGroupVersion(Scheme, schema.GroupVersion{Version: "v1"})

	// TODO: keep the generic API server from wanting this
	unversioned := schema.GroupVersion{Group: "", Version: "v1"}
	Scheme.AddUnversionedTypes(unversioned,
		&metav1.Status{},
		&metav1.APIVersions{},
		&metav1.APIGroupList{},
		&metav1.APIGroup{},
		&metav1.APIResourceList{},
	)
}

type Config struct {
	GenericConfig *genericapiserver.RecommendedConfig
	// SharedInformerFactory provides shared informers for resources
	SharedInformerFactory informers.SharedInformerFactory
}

// RunServer contains state for a Kubernetes cluster master/api server.
type RunServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

type CompletedConfig struct {
	*Config
	completedConfig *genericapiserver.CompletedConfig
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (c *Config) Complete() CompletedConfig {
	completedCfg := c.GenericConfig.Complete()

	c.GenericConfig.Version = &version.Info{
		Major: "1",
		Minor: "0",
	}

	return CompletedConfig{Config: c, completedConfig: &completedCfg}
}

// SkipComplete provides a way to construct a server instance without config completion.
func (c *Config) SkipComplete() CompletedConfig {
	completedCfg := c.GenericConfig.Complete()
	return CompletedConfig{Config: c, completedConfig: &completedCfg}
}

// New returns a new instance of RunServer from the given config.
func (c CompletedConfig) New() (*RunServer, error) {
	genericServer, err := c.completedConfig.New("run-apiserver", genericapiserver.EmptyDelegate)
	if err != nil {
		return nil, err
	}

	s := &RunServer{
		GenericAPIServer: genericServer,
	}

	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(aggregator.GroupName, registry, Scheme, metav1.ParameterCodec, Codecs)
	apiGroupInfo.GroupMeta.GroupVersion = v1.SchemeGroupVersion

	v1storage := map[string]rest.Storage{}
	v1storage["runs"] = aggregatorregistry.RESTInPeace(runStorage.NewREST(Scheme, c.GenericConfig.RESTOptionsGetter))
	apiGroupInfo.VersionedResourcesStorageMap["v1"] = v1storage

	if err := s.GenericAPIServer.InstallAPIGroup(&apiGroupInfo); err != nil {
		return nil, err
	}

	return s, nil
}
