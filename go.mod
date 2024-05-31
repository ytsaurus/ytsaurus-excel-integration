module go.ytsaurus.tech/yt/microservices/excel

go 1.22.0

require (
	github.com/c2h5oh/datasize v0.0.0-20220606134207-859f65c6625b
	github.com/go-chi/chi/v5 v5.0.12
	github.com/golang/protobuf v1.5.4
	github.com/stretchr/testify v1.9.0
	github.com/xuri/excelize/v2 v2.6.1
	go.uber.org/atomic v1.11.0
	go.uber.org/zap v1.27.0
	go.ytsaurus.tech/library/go/core/log v0.0.3
	go.ytsaurus.tech/library/go/core/metrics v0.0.1
	go.ytsaurus.tech/library/go/core/xerrors v0.0.3
	go.ytsaurus.tech/library/go/httputil/middleware/httpmetrics v0.0.1
	go.ytsaurus.tech/yt/go v0.0.18
	golang.org/x/sync v0.6.0
	golang.org/x/xerrors v0.0.0-20231012003039-104605ab7028
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/OneOfOne/xxhash v1.2.8 // indirect
	github.com/andybalholm/brotli v1.1.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v4 v4.2.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/frankban/quicktest v1.14.6 // indirect
	github.com/gofrs/uuid v4.2.0+incompatible // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/tink/go v1.7.0 // indirect
	github.com/klauspost/compress v1.17.8 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_golang v1.18.0 // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/common v0.46.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/richardlehane/mscfb v1.0.4 // indirect
	github.com/richardlehane/msoleps v1.0.3 // indirect
	github.com/rogpeppe/go-internal v1.12.0 // indirect
	github.com/xuri/efp v0.0.0-20220603152613-6918739fd470 // indirect
	github.com/xuri/nfp v0.0.0-20220409054826-5e722a1d9e22 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.ytsaurus.tech/library/go/blockcodecs v0.0.2 // indirect
	go.ytsaurus.tech/library/go/ptr v0.0.1 // indirect
	go.ytsaurus.tech/library/go/x/xreflect v0.0.2 // indirect
	go.ytsaurus.tech/library/go/x/xruntime v0.0.3 // indirect
	golang.org/x/crypto v0.21.0 // indirect
	golang.org/x/exp v0.0.0-20240222234643-814bf88cf225 // indirect
	golang.org/x/image v0.15.0 // indirect
	golang.org/x/net v0.23.0 // indirect
	golang.org/x/sys v0.18.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/protobuf v1.33.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/insomniacslk/dhcp => github.com/insomniacslk/dhcp v0.0.0-20210120172423-cc9239ac6294

// yo: update cloud.google.com/go/pubsub v1.30.0 => v1.32.0
// yo: failed to generate ya.make files for module "cloud.google.com/go/pubsub": cannot query module due to -mod=vendor
// (Go version in go.mod is at least 1.14 and vendor directory exists.)
replace cloud.google.com/go/pubsub => cloud.google.com/go/pubsub v1.30.0

replace google.golang.org/grpc => google.golang.org/grpc v1.56.3

// https://st.yandex-team.ru/TM-7347
replace github.com/jackc/pgtype => github.com/jackc/pgtype v1.12.0

replace github.com/aws/aws-sdk-go => github.com/aws/aws-sdk-go v1.46.7

replace k8s.io/api => k8s.io/api v0.26.1

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.26.1

replace k8s.io/apimachinery => k8s.io/apimachinery v0.26.1

replace k8s.io/apiserver => k8s.io/apiserver v0.26.1

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.26.1

replace k8s.io/client-go => k8s.io/client-go v0.26.1

replace k8s.io/cloud-provider => k8s.io/cloud-provider v0.26.1

replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.26.1

replace k8s.io/code-generator => k8s.io/code-generator v0.26.1

replace k8s.io/component-base => k8s.io/component-base v0.26.1

replace k8s.io/cri-api => k8s.io/cri-api v0.23.5

replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.26.1

replace k8s.io/dynamic-resource-allocation => k8s.io/dynamic-resource-allocation v0.26.1

replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.26.1

replace k8s.io/kube-proxy => k8s.io/kube-proxy v0.26.1

replace k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.26.1

replace k8s.io/kubelet => k8s.io/kubelet v0.26.1

replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.26.1

replace k8s.io/mount-utils => k8s.io/mount-utils v0.26.2-rc.0

replace k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.26.1

replace k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.26.1

// https://github.com/temporalio/features/blob/main/go.mod#L9 requires v1.0.0 which hasn't been released yet, so we replace for now
replace github.com/temporalio/features => github.com/temporalio/features v0.0.0-20231218231852-27c681667dae

replace github.com/temporalio/features/features => github.com/temporalio/features/features v0.0.0-20231218231852-27c681667dae

replace github.com/temporalio/features/harness/go => github.com/temporalio/features/harness/go v0.0.0-20231218231852-27c681667dae

replace github.com/temporalio/omes => github.com/temporalio/omes v0.0.0-20240429210145-5fa5c107b7a8

// https://github.com/goccy/go-yaml/issues/413
replace github.com/goccy/go-yaml => github.com/goccy/go-yaml v1.9.5

replace github.com/aleroyer/rsyslog_exporter => github.com/prometheus-community/rsyslog_exporter v1.1.0

// Workaround weird go.mod shipped with k8s.io submodules.
// For the reasoning see
// https://suraj.io/post/2021/05/k8s-import/
//
// The list was generated automatically with the following script:
// https://github.com/kubernetes/kubernetes/issues/79384#issuecomment-521493597
//
// You can also use the following command:
// ya grep --remote -f='vendor/k8s.io/.*/go.mod' 'v0.0.0$' | cut -d : -f 3 | sort | uniq
