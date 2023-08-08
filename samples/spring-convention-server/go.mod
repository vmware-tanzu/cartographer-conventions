module github.com/vmware-tanzu/cartographer-conventions/samples/spring-convention-server

go 1.19

replace github.com/vmware-tanzu/cartographer-conventions/webhook => ../../webhook

require (
	github.com/CycloneDX/cyclonedx-go v0.6.0
	// wokeignore:rule=Masterminds
	github.com/Masterminds/semver v1.5.0
	github.com/go-logr/logr v1.2.4
	github.com/go-logr/zapr v1.2.4
	github.com/vmware-tanzu/cartographer-conventions/webhook v0.2.0
	go.uber.org/zap v1.25.0
	k8s.io/api v0.27.4
	k8s.io/apimachinery v0.27.4
)

require (
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/go-containerregistry v0.15.2 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/net v0.10.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/klog/v2 v2.90.1 // indirect
	k8s.io/utils v0.0.0-20230209194617-a36077c30491 // indirect
	sigs.k8s.io/controller-runtime v0.15.0 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
)
