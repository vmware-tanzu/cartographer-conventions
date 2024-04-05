//go:build tools
// +build tools

// This package imports things required by build scripts, to force `go mod` to see them as dependencies
package tools

import (
	_ "github.com/k14s/ytt/cmd/ytt"
	_ "golang.org/x/tools/cmd/goimports"
	_ "reconciler.io/dies/diegen"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
	_ "sigs.k8s.io/kustomize/kustomize/v5"
)
