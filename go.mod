module pgtest

go 1.13

require (
	github.com/lib/pq v1.0.1-0.20181016162627-9eb73efc1fcc
	github.com/mxk/go-sqlite v0.0.0-20140611214908-167da9432e1f
	gopkg.in/yaml.v2 v2.2.2
)

replace github.com/lib/pq => /home/lhq/go/src/github.com/pq
