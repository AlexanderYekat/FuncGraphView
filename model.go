package main

type BackendType string

type initResp struct {
	EdgesCount  int               `json:"edgesCount"`
	NodesCount  int               `json:"nodesCount"`
	Product     string            `json:"product"`
	Categories  map[string]string `json:"categories"`
	BackendType BackendType       `json:"backendType"`
}

const (
	BackendTypeDB   = "neo4j-db"
	BackendTypeBolt = "neo4j-bolt"
	BackendTypeGSON = "neo4j-gson"
)

type Node struct {
	Label      string   `json:"label"`
	Id         int      `json:"id"`
	Categories []string `json:"categories"`
	Value      int      `json:"value"`
	Group      string   `json:"group"`
	Image      string   `json:"image,omitempty"`
	x          float64  `json:"x"`
	y          float64  `json:"y"`
}

type Edge struct {
	Id    int    `json:"id"`
	Label string `json:"label"`
	From  int    `json:"from"`
	To    int    `json:"to"`
}

type loadGraphResp struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

type params struct {
	NodeIds []int  `json:"nodeIds,omitempty"`
	Expr    string `json:"expr,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

type nodesInfoResp struct {
	Infos []string `json:"infos"`
}
