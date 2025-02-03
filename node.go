package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/rpc"
	"os"
	"strconv"
	"sync"
	"time"
)

type HBTable struct {
    NodeID int
    Status int
    HBcount int64
    Timestamp time.Time
}

type Args struct {
    NodeID int
    HBTableList []HBTable
}

type HBServer int64

// Global Heartbeat Channel
var hbChannel = make(chan Args, 50)

func generate_id() int {
    rand.New(rand.NewSource(time.Now().Unix()))
    randInt := rand.Intn(9000) + 1000

    return randInt
}

func port_picker(arg1 string, arg2 string) string {
    rand.New(rand.NewSource(time.Now().Unix()))
    randint := rand.Intn(2)
    if randint == 1 {
        return arg1
    } else {
        return arg2
    }
}

func update_tablelist(hb_tablelist *[]HBTable, client_hb_tablelist []HBTable, cnodeid int, mu *sync.Mutex) {
    // Check if client node is in the table list
    log.Println("Checking if client is in our records")
    exists := false

    // First check if the node exists in hb_tablelist
    for oindex := range (*hb_tablelist) {
        if (*hb_tablelist)[oindex].NodeID == cnodeid {
            exists = true
            break
        }
    }

    if !exists {
        log.Println("Client is not in our records, adding now...")
        // Add the node from client_hb_tablelist if it doesn't exist in hb_tablelist
        for cindex := range (client_hb_tablelist) {
            if (client_hb_tablelist)[cindex].NodeID == cnodeid {
                mu.Lock()
                *hb_tablelist = append(*hb_tablelist, (client_hb_tablelist)[cindex])
                (*hb_tablelist)[len((*hb_tablelist))-1].HBcount++
                (*hb_tablelist)[len((*hb_tablelist))-1].Status = 0
                (*hb_tablelist)[len((*hb_tablelist))-1].Timestamp = time.Now()
                mu.Unlock()
                break
            }
        }
    } else {
        log.Println("Client exists in our records, updating node data...")
        // Update the existing node in hb_tablelist
        for oindex := range (*hb_tablelist) {
            if (*hb_tablelist)[oindex].NodeID == cnodeid {
                mu.Lock()
                (*hb_tablelist)[oindex].HBcount++
                (*hb_tablelist)[oindex].Status = 0
                (*hb_tablelist)[oindex].Timestamp = time.Now()
                mu.Unlock()
                break
            }
        }
    }

    // Now update the table with client_hb_tablelist data
    log.Println("Updating our tableslice with entries from client tableslice")
    for cindex := range client_hb_tablelist {
        clientTableNode := (client_hb_tablelist)[cindex]
        exists := false

        // Check if the client node already exists in hb_tablelist
        for oindex := range *hb_tablelist {
            if (*hb_tablelist)[oindex].NodeID == clientTableNode.NodeID {
                exists = true
                if !clientTableNode.Timestamp.Before((*hb_tablelist)[oindex].Timestamp) {
                    // Replace the existing node
                    mu.Lock()
                    (*hb_tablelist)[oindex] = clientTableNode
                    mu.Unlock()
                    log.Printf("Updated node with ID: %d\n", clientTableNode.NodeID)
                }
                break
            }
        }

        // If the node doesn't exist, append it to hb_tablelist
        if !exists {
            mu.Lock()
            *hb_tablelist = append(*hb_tablelist, clientTableNode)
            mu.Unlock()
            log.Printf("Added node with ID: %d\n", clientTableNode.NodeID)
        }
    }

    log.Println("Checking for failing nodes...")
    var toRemove []int // To track indices for removal
    for oindex := range *hb_tablelist {
        subbed_time := time.Now().Sub((*hb_tablelist)[oindex].Timestamp)
        if subbed_time.Seconds() > 60 {
            toRemove = append(toRemove, oindex) // Mark for removal
            log.Printf("Node %d marked to be purged\n", (*hb_tablelist)[oindex].NodeID)
        } else if subbed_time.Seconds() > 30 {
            mu.Lock()
            (*hb_tablelist)[oindex].Status = 1 // Set status to 1 if older than 30s 
            mu.Unlock()
            log.Printf("Node %d marked as failing\n", (*hb_tablelist)[oindex].NodeID)
        } 
    }

    // Remove elements from client_hb_tablelist in reverse order
    for i := len(toRemove) - 1; i >= 0; i-- {
        index := toRemove[i]
        mu.Lock()
        *hb_tablelist = append((*hb_tablelist)[:index], (*hb_tablelist)[index+1:]...)
        mu.Unlock()
    }
}

func (t *HBServer) Heartbeat(args *Args, reply *string) error {
    log.Println("Client message received!")
    hbChannel<-*args
    return nil
}

func create_hbserver(arg string) {
    // Create a new RPC server
    hbserver := new(HBServer)
    // Register RPC server
    rpc.Register(hbserver)
    l, e := net.Listen("tcp", ":" + arg)
    if e != nil {
        log.Fatal("listen error:", e)
    }
    defer l.Close()

    log.Println("Node listening on port: " + arg)

    var wg sync.WaitGroup
    for {
        conn, err := l.Accept()
        if err != nil {
            log.Printf("Accept error: %v\n", err)
            continue
        }
        wg.Add(1)
        go func() {
            defer wg.Done()
            rpc.ServeConn(conn)
            conn.Close()
        }()
    }
}

func main() {
    go create_hbserver(os.Args[1])
    node_id := generate_id()

    var owntable HBTable = HBTable{
        NodeID: node_id,
        Status: 0,
        HBcount: 0,
        Timestamp: time.Now(),
    }

    var hbtablelist []HBTable = []HBTable{}
    hbtablelist = append(hbtablelist, owntable)

    var mu sync.Mutex

    cyclecounter := 0

    for {
        // Simulate node failing and then revive
        if (len(os.Args) == 5) {
            if (os.Args[4] == "bad") && (cyclecounter > 20) {
                break
            }
        }

        client, err := rpc.Dial("tcp", "localhost:" + port_picker(os.Args[2], os.Args[3]))
        if err != nil {
            log.Println("dialing:", err)
            time.Sleep(1 * time.Second)
            continue
        }

        args := Args{
            NodeID: node_id,
            HBTableList: hbtablelist,
        }

        call := client.Go("HBServer.Heartbeat", args, nil, nil)
        select {
        case <-call.Done:
            if call.Error != nil {
                log.Println("RPC call failed:", call.Error)
                log.Println("Retrying...")
                continue
            }
        case <-time.After(10 * time.Second):
            log.Println("RPC call timed out")
            log.Println("Retrying...")
            continue
        }

        argholder := <-hbChannel

        update_tablelist(&hbtablelist, argholder.HBTableList, argholder.NodeID, &mu)

        fmt.Println("\n\nTable List for Node: " + strconv.Itoa(node_id))
        fmt.Println("-------------------------------------")
        for _, value := range hbtablelist {
            fmt.Printf("NodeID: %d\n", value.NodeID)
            fmt.Printf("Status: %d\n", value.Status)
            fmt.Printf("HBCount: %d\n", value.HBcount)
            fmt.Println("Timestamp: " + value.Timestamp.Format("15:04:05"))
            fmt.Println()
        }
        fmt.Println("-------------------------------------")
        fmt.Println()

        cyclecounter += 1

        time.Sleep(1 * time.Second)
    }
}
