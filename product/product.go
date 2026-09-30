// Package product names the product lines that share the puckoops platform.
//
// Products share each environment's GCP project, cluster, bucket and MongoDB
// cluster, so they are kept apart by name: a product's objects live under
// "<product>/" in the bucket, its collections in the "<product>" database, and
// its permissions in the "<product>" OpenFGA store. A new product is added here
// and to PRODUCTS in gke-infra-bootstrap's platform.sh before its first service.
package product

import (
	"fmt"
	"path"
	"strings"
)

type Product string

const (
	Orbit    Product = "orbit"
	Puckoops Product = "puckoops"
)

func All() []Product { return []Product{Orbit, Puckoops} }

// Parse is exact: "Orbit" is not a product, because the name is also a bucket
// prefix and a database name, where case matters.
func Parse(name string) (Product, error) {
	for _, p := range All() {
		if string(p) == name {
			return p, nil
		}
	}
	return "", fmt.Errorf("product: unknown product %q", name)
}

func (p Product) String() string { return string(p) }

func (p Product) Database() string { return string(p) }

func (p Product) FGAStore() string { return string(p) }

// ObjectPrefix is the part of the shared bucket the product's services may
// touch; their bucket roles are conditioned on it.
func (p Product) ObjectPrefix() string { return string(p) + "/" }

// ObjectKey puts a key under the product's prefix. The parts are cleaned
// before the prefix goes on, so ".." cannot climb into another product.
func (p Product) ObjectKey(parts ...string) string {
	return p.ObjectPrefix() + strings.TrimPrefix(path.Clean("/"+path.Join(parts...)), "/")
}
