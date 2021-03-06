package iface

import (
	"strconv"

	"github.com/graphql-go/graphql"
	"github.com/usnistgov/ndn-dpdk/core/gqlserver"
	"github.com/usnistgov/ndn-dpdk/dpdk/eal"
)

func init() {
	ntFace := gqlserver.NewNodeType((*Face)(nil))
	tFace := graphql.NewObject(ntFace.Annotate(graphql.ObjectConfig{
		Name: "Face",
		Fields: graphql.Fields{
			"nid": &graphql.Field{
				Type:        gqlserver.NonNullInt,
				Description: "Numeric face identifier.",
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					face := p.Source.(Face)
					return int(face.ID()), nil
				},
			},
			"locator": &graphql.Field{
				Type:        gqlserver.JSON,
				Description: "Endpoint addresses.",
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					face := p.Source.(Face)
					return face.Locator(), nil
				},
			},
			"numaSocket": eal.GqlWithNumaSocket,
			"counters": &graphql.Field{
				Type:        gqlserver.JSON,
				Description: "Face counters.",
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					face := p.Source.(Face)
					return face.ReadCounters(), nil
				},
			},
		},
	}))
	ntFace.Retrieve = func(id string) (interface{}, error) {
		nid, e := strconv.Atoi(id)
		if e != nil {
			return nil, e
		}
		return Get(ID(nid)), nil
	}
	ntFace.Delete = func(source interface{}) error {
		face := source.(Face)
		return face.Close()
	}
	ntFace.Register(tFace)

	gqlserver.AddQuery(&graphql.Field{
		Name:        "faces",
		Description: "List of faces.",
		Type:        graphql.NewList(tFace),
		Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			return List(), nil
		},
	})

	gqlserver.AddMutation(&graphql.Field{
		Name:        "createFace",
		Description: "Create a face.",
		Args: graphql.FieldConfigArgument{
			"locator": &graphql.ArgumentConfig{
				Type: gqlserver.JSON,
			},
		},
		Type: tFace,
		Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			var locw LocatorWrapper
			if e := gqlserver.DecodeJSON(p.Args["locator"], &locw); e != nil {
				return nil, e
			}
			return locw.Locator.CreateFace()
		},
	})
}
