module github.com/johbar/text-extraction-service/v2

go 1.21

require (
	github.com/bytedance/sonic v1.10.2
	github.com/chenzhuoyu/base64x v0.0.0-20230717121745-296ad89f973d
	github.com/dlclark/regexp2 v1.10.0
	github.com/gabriel-vasile/mimetype v1.4.3
	github.com/gen2brain/go-fitz v1.23.7
	github.com/gin-contrib/expvar v0.0.1
	github.com/gin-gonic/gin v1.9.1
	github.com/go-playground/validator/v10 v10.17.0
	github.com/johbar/go-poppler v1.0.1-0.20230721224815-a1860a344718
	github.com/nats-io/nats-server/v2 v2.10.10
	github.com/nats-io/nats.go v1.32.0
	github.com/richardlehane/mscfb v1.0.4
	github.com/richardlehane/msoleps v1.0.3
	github.com/samber/slog-gin v1.10.1
	github.com/spf13/viper v1.18.2
	golang.org/x/exp v0.0.0-20240119083558-1b970713d09a
	golang.org/x/text v0.14.0
)

require (
	github.com/chenzhuoyu/iasm v0.9.1 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.5 // indirect
	github.com/klauspost/cpuid/v2 v2.2.6 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/minio/highwayhash v1.0.2 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/nats-io/jwt/v2 v2.5.3 // indirect
	github.com/nats-io/nkeys v0.4.7 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/pelletier/go-toml/v2 v2.1.1 // indirect
	github.com/sagikazarmark/locafero v0.4.0 // indirect
	github.com/sagikazarmark/slog-shim v0.1.0 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spf13/afero v1.11.0 // indirect
	github.com/spf13/cast v1.6.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.12 // indirect
	go.opentelemetry.io/otel v1.22.0 // indirect
	go.opentelemetry.io/otel/trace v1.22.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/arch v0.7.0 // indirect
	golang.org/x/crypto v0.18.0 // indirect
	golang.org/x/net v0.20.0 // indirect
	golang.org/x/sys v0.16.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	google.golang.org/protobuf v1.32.0 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/nats-io/nats.go v1.32.0 => github.com/johbar/nats.go v0.0.0-20240128001054-5b9aad82c9d7
