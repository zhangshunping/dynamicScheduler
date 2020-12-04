module dynamicScheduler

go 1.14

require (
	github.com/antonfisher/nested-logrus-formatter v1.2.0
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/prometheus/client_golang v1.7.1
	github.com/sirupsen/logrus v1.4.2
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d // indirect
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e // indirect
	k8s.io/api v0.18.0
	k8s.io/apimachinery v0.19.1
	k8s.io/client-go v0.18.0
	k8s.io/utils v0.0.0-20200821003339-5e75c0163111 // indirect

)

replace github.com/googleapis/gnostic v0.5.3 => github.com/googleapis/gnostic v0.1.0
