package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"hash/fnv"

	"github.com/LazarenkoA/1c-language-parser/ast"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

var (
	utf8BOM = []byte{0xEF, 0xBB, 0xBF}
)

const rootPath = "./TestCommonModules"

func main() {
	fmt.Println("=== Starting application ===")
	fmt.Printf("Working directory: %s\n", rootPath)

	trees, err := walkDir(rootPath)
	fmt.Printf("Found %d files to parse\n", len(trees))
	if err != nil {
		fmt.Printf("Error walking directory: %v\n", err)
		return
	}

	nodes := buildNodes(trees)
	fmt.Printf("Built %d nodes with %d edges\n", len(nodes.Nodes), len(nodes.Edges))

	nodesFor3D := buildNodesFor3D(trees)
	fmt.Printf("Built 3D graph with %d nodes and %d links\n",
		len(nodesFor3D.Nodes), len(nodesFor3D.Links))

	r := gin.Default()
	fmt.Println("=== Server configuration ===")
	fmt.Println("Setting up routes:")
	fmt.Println("- GET /graphserver")
	fmt.Println("- GET /json")
	fmt.Println("- GET /")

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	r.GET("/graphserver", func(c *gin.Context) {
		command := c.Query("command")
		if command == "" {
			c.Status(http.StatusBadRequest)
			return
		}

		var param params
		if err := c.ShouldBindJSON(&param); err != nil {
			// Игнорируем ошибку, так как параметры могут быть пустыми
		}

		if data, err := invokeIGPCommand(command, nodes, &param); err == nil {
			c.Data(http.StatusOK, "application/json", data)
		} else {
			c.Status(http.StatusInternalServerError)
		}
	})

	r.GET("/json", func(c *gin.Context) {
		c.JSON(http.StatusOK, nodesFor3D)
	})

	r.GET("/", func(c *gin.Context) {
		c.File("index3D.html")
	})

	fmt.Println("=== Server started at :8080 ===")
	r.Run(":8080")
}

func invokeIGPCommand(command string, graph *loadGraphResp, param *params) ([]byte, error) {
	switch strings.ToLower(command) {
	case "init":
		resp := initResp{
			EdgesCount:  len(graph.Edges),
			NodesCount:  len(graph.Nodes),
			Product:     "Go Demo",
			Categories:  map[string]string{"notuse": "не используемые"},
			BackendType: BackendTypeGSON,
		}

		return json.Marshal(&resp)
	case "loadgraph":
		return json.Marshal(graph)
	case "search":
		filtered := lo.Filter(graph.Nodes, func(item Node, index int) bool {
			return item.Label == param.Expr
		})

		limit := int(math.Min(float64(param.Limit), float64(len(filtered))))
		resp := loadGraphResp{Nodes: filtered[:limit]}
		return json.Marshal(&resp)
	case "getnodesinfo":
		resp := nodesInfoResp{
			Infos: []string{},
		}

		filtered := lo.Filter(graph.Nodes, func(item Node, index int) bool {
			return lo.Some(param.NodeIds, []int{item.Id})
		})

		for _, n := range filtered {
			resp.Infos = append(resp.Infos, fmt.Sprintf("<div style=\"word-wrap: break-word; padding: 10px;\"><p>%s</p></div>", n.Label))
		}

		return json.Marshal(&resp)
	}

	return nil, nil
}

func parseFile(filePath string) (*ast.AstNode, error) {
	fmt.Printf("  Parsing file: %s\n", filePath)

	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer f.Close()

	data, _ := io.ReadAll(f)
	fmt.Printf("  File size: %d bytes\n", len(data))

	if bytes.HasPrefix(data, utf8BOM) {
		data = data[len(utf8BOM):]
		fmt.Println("  BOM detected and removed")
	}

	ast := ast.NewAST(string(data))
	fmt.Println("  AST created, starting parse")

	if err := ast.Parse(); err != nil {
		return nil, fmt.Errorf("parse error: %v", err)
	}
	fmt.Println("  Parse completed successfully")

	s := strings.Split(filePath, string(os.PathSeparator))
	if len(s) < 3 {
		fmt.Printf("bad file path %s", filePath)
	} else {
		ast.ModuleStatement.Name = s[len(s)-3]
	}

	return ast, nil
}

func buildNodesFor3D(trees []*ast.AstNode) *Graph3DResp {
	nodes := buildNodes(trees)
	result := new(Graph3DResp)

	for _, n := range nodes.Nodes {
		result.Nodes = append(result.Nodes, Node3D{
			Id:          strconv.Itoa(n.Id),
			Group:       int(HashStringToInt(n.Group)),
			Description: n.Label,
			Value:       n.Value,
		})
	}

	for _, e := range nodes.Edges {
		result.Links = append(result.Links, Edges3D{
			Source: strconv.Itoa(e.From),
			Target: strconv.Itoa(e.To),
		})
	}

	return result
}

func buildNodes(trees []*ast.AstNode) *loadGraphResp {
	result := new(loadGraphResp)

	type funcInfo struct {
		stCount    int
		inRefCount int
		id         int
		dependence []string
		export     bool
		moduleName string
		notUse     bool
	}

	pf := map[string]funcInfo{}

	for _, m := range trees {
		fmt.Println("ModuleStatement", m.ModuleStatement.Name)
		fmt.Println("ModuleStatement SrsCode", m.SrsCode())
		json, _ := m.JSON()
		fmt.Printf("ModuleStatement JSON %v\n", string(json))
		m.ModuleStatement.Walk(func(currentFP *ast.FunctionOrProcedure, statement *ast.Statement) {
			fmt.Println("Walk FunctionOrProcedure", currentFP.Name)
			fmt.Printf("Walk statement %T %v\n", *statement, *statement)
			fmt.Println("Walk statement", statement)
			if currentFP == nil {
				fmt.Println("currentFP is nil")
				return
			}

			key := m.ModuleStatement.Name + "." + currentFP.Name
			if _, ok := pf[key]; !ok {
				pf[key] = funcInfo{id: len(pf), export: currentFP.Export, notUse: true, moduleName: m.ModuleStatement.Name}
			}

			v := pf[key]

			switch value := (*statement).(type) {
			case ast.MethodStatement:
				fmt.Println("MethodStatement", value.Name)
				v.dependence = lo.Union(v.dependence, []string{m.ModuleStatement.Name + "." + value.Name})
			case ast.CallChainStatement:
				if value.IsMethod() {
					fmt.Println("CallChainStatement", value.Call)
					v.dependence = append(v.dependence, printCallChainStatement(value))
				}
			case *ast.BuiltinFunctionStatement:
				fmt.Println("BuiltinFunctionStatement", value.Name)
				v.dependence = append(v.dependence, value.Name)
			default:
				fmt.Printf("Unknown statement %T\n", *statement)
				fmt.Println("Unknown statement", statement)
			}

			if f, ok := (*statement).(*ast.FunctionOrProcedure); ok {
				v.stCount = len(f.Body) + 1
			}

			pf[key] = v
		})

	}

	var edgesID int
	for name, v := range pf {
		result.Nodes = append(result.Nodes, Node{
			Label: name,
			Id:    v.id,
			Value: v.stCount,
			Group: v.moduleName, //fmt.Sprintf("%v", v.export),
		})

		for _, d := range v.dependence {
			to, ok := pf[d]
			if !ok {
				continue
			}

			result.Edges = append(result.Edges, Edge{
				Id:   edgesID,
				From: v.id,
				To:   to.id,
			})

			to.notUse = false
			to.inRefCount++
			edgesID++

			pf[d] = to
		}

		//result.Nodes[len(result.Nodes)-1].Value = v.inRefCount
		if v.inRefCount > 0 {
			result.Nodes[len(result.Nodes)-1].Value *= v.inRefCount
		}
	}

	for i, n := range result.Nodes {
		if pf[n.Label].notUse {
			result.Nodes[i].Categories = append(result.Nodes[i].Categories, "notuse")
		}
	}

	return result
}

func printCallChainStatement(call ast.Statement) (result string) {
	switch v := call.(type) {
	case ast.CallChainStatement:
		if v.Call != nil {
			return printCallChainStatement(v.Call) + "." + printCallChainStatement(v.Unit)
		}
	case ast.VarStatement:
		return v.Name
	case ast.MethodStatement:
		return v.Name
	}

	return
}

func walkDir(rootPath string) ([]*ast.AstNode, error) {
	fmt.Printf("\nScanning directory: %s\n", rootPath)
	result := make([]*ast.AstNode, 0)

	err := filepath.WalkDir(rootPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && filepath.Ext(path) == ".bsl" {
			fmt.Printf("Processing file: %s\n", path)
			a, err := parseFile(path)
			if err != nil {
				fmt.Printf("Error parsing file %s: %v\n", path, err)
				return nil
			}
			result = append(result, a)
			fmt.Printf("Successfully parsed: %s\n", path)
		}
		return nil
	})

	return result, err
}

func HashStringToInt(s string) uint64 {
	h := fnv.New64a() // Используем FNV-1a 64-битный хешер
	h.Write([]byte(s))
	return h.Sum64() // Возвращаем хеш в виде uint64
}
