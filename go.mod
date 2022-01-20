module github.com/stolostron/multicloud-operators-deployable

go 1.15

require (
	github.com/cameront/go-jsonpatch v0.0.0-20180223123257-a8710867776e
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/go-openapi/spec v0.19.4
	github.com/onsi/gomega v1.10.1
	github.com/open-cluster-management/api v0.0.0-20201007180356-41d07eee4294
	github.com/open-cluster-management/multicloud-operators-placementrule v1.2.2-2-20201130-98cfd
	github.com/spf13/pflag v1.0.5
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	golang.org/x/net v0.0.0-20211112202133-69e39bad7dc2
	k8s.io/api v0.20.0
	k8s.io/apiextensions-apiserver v0.19.3
	k8s.io/apimachinery v0.20.0
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/klog v1.0.0
	k8s.io/kube-openapi v0.0.0-20201113171705-d219536bb9fd
	sigs.k8s.io/controller-runtime v0.6.3
)

require golang.org/x/crypto v0.0.0-20211215153901-e495a2d5b3d3 // indirect

replace (
	github.com/open-cluster-management/api => open-cluster-management.io/api v0.0.0-20210513122330-d76f10481f05
	github.com/open-cluster-management/multicloud-operators-placementrule => github.com/stolostron/multicloud-operators-placementrule v1.2.2-2-20201130-98cfd
	k8s.io/api => k8s.io/api v0.19.3
	k8s.io/client-go => k8s.io/client-go v0.19.3
)
