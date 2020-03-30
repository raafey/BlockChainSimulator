package main

import (
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

var serverPort string
var recvdConns []net.Conn
var dialedConns []net.Conn
var blockChain *Block
var peers []string

type Block struct {
	Transaction string
	PrevPointer *Block
	PrevHash    [32]byte
}

type Message struct {
	ChainHead *Block
	PeersAddr []string
}

type Peer struct {
	Conn net.Conn
	Port string
}

func mintBlock(Transaction string) *Block {
	var newBlock *Block

	if blockChain == nil {
		newBlock = &Block{Transaction: Transaction, PrevPointer: nil}
	} else {
		hashValue := sha256.Sum256(append([]byte(blockChain.Transaction)[:], blockChain.PrevHash[:]...))
		newBlock = &Block{Transaction, blockChain, hashValue}
	}
	fmt.Println("\nNew Block: " + newBlock.Transaction + " added!\n")

	return newBlock
}

func listBlocks(chainHead *Block) {
	println("\n\nListing Current Blocks in Chain: ")
	current := chainHead
	blockNum := 0
	for {
		if current == nil {
			break
		}
		fmt.Println("\nBlock: " + strconv.Itoa(blockNum))
		fmt.Println("Transactions:")
		trans := strings.Split(current.Transaction, "\n")

		i := 1
		for _, t := range trans {
			fmt.Println(strconv.Itoa(i) + ": " + t)
			i++
		}
		fmt.Print("PrevHash: ")
		fmt.Printf("%x\n", current.PrevHash)

		current = current.PrevPointer
		blockNum++
	}
	printMenu(peers)
}

func getConnection(c net.Conn, peer chan Peer) {
	log.Println("A Peer has connected at ", c.RemoteAddr())

	buffer := make([]byte, 4096)
	n, err := c.Read(buffer)
	if err != nil || n == 0 {
		c.Close()
	}
	clientPort := string(buffer[0:n])
	fmt.Print("Client Listening at port: " + clientPort)
	peer <- Peer{c, clientPort}
}

func distributeChain(c net.Conn, connections []Peer) {
	ports := []string{}
	for _, peer := range connections {
		if c != peer.Conn {
			ports = append(ports, peer.Port)
		}
	}

	gobEncoder := gob.NewEncoder(c)
	err := gobEncoder.Encode(&Message{blockChain, ports})
	if err != nil {
		log.Println(err)
	}
}

func connectToPeer(serverPort string) net.Conn {
	conn, err := net.Dial("tcp", "localhost:"+serverPort)
	if err != nil {
		fmt.Println(err)
	}

	return conn
}

func printMenu(peers []string) {
	if len(peers) > 0 {
		fmt.Println("\n+-------------Menu-------------+")

		for i := 0; i < len(peers); i++ {
			fmt.Println(strconv.Itoa(i) + ". Peer: " + peers[i])
		}
		fmt.Println("+-------------------------------+")
		fmt.Print(serverPort + "'s Current Balance: ")
		fmt.Println(getBalance(serverPort))
		fmt.Print("Choose peer for transaction: ")
	}
}

func pingMiner(message string, peerConn net.Conn) bool {
	command := []string{}
	command = append(command, "MINE")
	command = append(command, message)

	gobEncoder := gob.NewEncoder(peerConn)
	err := gobEncoder.Encode(&Message{nil, command})
	if err != nil {
		log.Println(err)
	}

	buffer := make([]byte, 4096)
	n, err := peerConn.Read(buffer)
	if err != nil || n == 0 {
		peerConn.Close()
	}
	msg := string(buffer[0:n])

	flag := false
	if msg == "T" {
		flag = true
	}

	return flag
}

func broadcastChain(chainHead *Block) {
	fmt.Println("Broadcasting updated chain to Peers!")
	command := []string{}
	command = append(command, "UPDATE_CHAIN")

	for _, peerConn := range dialedConns {
		gobEncoder := gob.NewEncoder(peerConn)
		err := gobEncoder.Encode(&Message{chainHead, command})
		if err != nil {
			log.Println(err)
		}
	}
}

func handleTransactions(peerConns []net.Conn, peers []string) {
	flag := true
	count := 0
	for {
		rand.Seed(time.Now().UnixNano())
		max := len(peers) - 1
		min := 0
		var option int
		var amount float64

		if flag == true {
			flag = false
			option := rand.Intn(max - min + 1)
			fmt.Println("Satoshi sending 50 coins to Peer: " + peers[option])
			amount = 50

		} else {
			if count == 1 {
				printMenu(peers)
			}

			fmt.Scan(&option)
			fmt.Print("Enter amount you want to send: ")
			fmt.Scan(&amount)
		}

		if option >= 0 && option < len(peers) {
			minerIdx := option
			for minerIdx == option {
				minerIdx = rand.Intn(max - min + 1)
			}

			fmt.Println(peers[minerIdx] + " selected as miner for transaction!")

			amt := amount
			for amt <= 0 {
				fmt.Println("Invalid amount entered! Please Enter again!")
				fmt.Println("Enter amount you want to send: ")
				fmt.Scan(&amount)
				amt = amount
			}

			message := serverPort + "-" + peers[option] + ":" + fmt.Sprintf("%2f", amount)
			status := pingMiner(message, peerConns[minerIdx])
			fmt.Println("Waiting for response from miner: " + peers[minerIdx] + "...")
			if status == true {
				fmt.Println("Transaction successful!")
			} else {
				fmt.Println("Transaction failed due to insufficient balance!")
			}

		} else {
			fmt.Println("Invalid input!")
		}
		count++
	}
}

func verifyBlock(chainHead *Block) bool {
	current := chainHead
	flag := true

	if current.PrevPointer != nil {
		PrevHash := sha256.Sum256(append([]byte(current.PrevPointer.Transaction)[:], current.PrevPointer.PrevHash[:]...))

		if current.PrevHash != PrevHash {
			fmt.Println("Hashes b/w block: " + current.Transaction + " and block: " + current.PrevPointer.Transaction + " don't match!")
			flag = false
		} else {
			fmt.Println("Hashes b/w block: " + current.Transaction + " and block: " + current.PrevPointer.Transaction + " match!")
		}
	}

	if flag {
		println("Chain verified! new block is valid")
	} else {
		println("Chain could not be verified! new block is invalid")
	}

	return flag
}

func handlePeerRequest(c net.Conn) {
	time.Sleep(time.Second * 2)
	for {
		var rcvdMessage Message
		dec := gob.NewDecoder(c)
		err := dec.Decode(&rcvdMessage)
		if err != nil {
			fmt.Println(err)
		}
		chainHead := rcvdMessage.ChainHead
		if rcvdMessage.PeersAddr[0] == "MINE" {
			substrs := strings.Split(rcvdMessage.PeersAddr[1], ":")
			amount := substrs[1]
			substrs = strings.Split(substrs[0], "-")
			sender := substrs[0]
			fmt.Println("\nI am mining now!")
			fmt.Println("Sender: " + sender)
			fmt.Println("Amount: " + amount)
			balance := getBalance(sender)
			fmt.Print(sender + "'s Balance: ")
			fmt.Println(balance)

			hasBalance := false
			amt, err := strconv.ParseFloat(amount, 64)
			if amt <= balance {
				hasBalance = true
			}

			if hasBalance == true {
				fmt.Println(sender + " has sufficient balance for transaction!")
				hashValue := sha256.Sum256(append([]byte(blockChain.Transaction)[:], blockChain.PrevHash[:]...))
				var newBlock *Block
				minerTransaction := "\nnil-" + serverPort + ":12.5"
				newBlock = &Block{rcvdMessage.PeersAddr[1] + minerTransaction, blockChain, hashValue}
				blockChain = newBlock

				_, err = c.Write([]byte("T"))
				if err != nil {
					c.Close()
					fmt.Println(err)
				}
				fmt.Println("Broadcasting updated block to Peers!")
				broadcastChain(blockChain)
				fmt.Println("Updated Chain:")
				listBlocks(blockChain)
			} else {
				fmt.Println(sender + " has insufficient balance for transaction!")
				_, err = c.Write([]byte("F"))
				if err != nil {
					c.Close()
					fmt.Println(err)
				}
			}

		} else if rcvdMessage.PeersAddr[0] == "UPDATE_CHAIN" {
			fmt.Println("\nReceived updated chain!")
			isValid := verifyBlock(chainHead)

			if isValid == true {
				blockChain = chainHead
				fmt.Println("Updated Chain:")
				listBlocks(blockChain)
			}
		}
	}
}

func getAmount(transactions string, sender string) float64 {
	trans := strings.Split(transactions, "\n")
	amt := 0.0
	for _, t := range trans {
		substrs := strings.Split(t, ":")
		party := strings.Split(substrs[0], "-")

		amount, err := strconv.ParseFloat(substrs[1], 64)
		if err != nil {
			fmt.Println(err)
		}

		if sender == party[0] {
			amount *= -1.0
		} else if sender == party[1] {
			amount *= 1.0
		} else {
			amount = 0.0
		}

		amt += amount
	}
	return amt
}

func getBalance(sender string) float64 {
	current := blockChain
	balance := 0.0
	for current != nil {
		balance += getAmount(current.Transaction, sender)
		current = current.PrevPointer
	}

	return balance
}

func main() {
	numClients, err := strconv.Atoi(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	if numClients < 2 {
		fmt.Println("Num of peers should be >= 2!")
		os.Exit(1)
	}

	serverPort = os.Args[2]
	ln, err := net.Listen("tcp", ":"+serverPort)
	if err != nil {
		log.Fatal(err)
	}
	blockChain = nil
	blockChain = mintBlock("nil-" + serverPort + ":100")

	count := 0
	connections := []Peer{}
	readPeer := make(chan Peer)
	for {
		if count == numClients {
			break
		}

		conn, err := ln.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go getConnection(conn, readPeer)

		p := <-readPeer
		connections = append(connections, p)
		recvdConns = append(recvdConns, p.Conn)

		blockChain = mintBlock("nil-" + p.Port + ":0")
		count++
	}

	listBlocks(blockChain)

	for _, conn := range recvdConns {
		go handlePeerRequest(conn)
	}
	peers = []string{}
	for _, peer := range connections {
		go distributeChain(peer.Conn, connections)
		peers = append(peers, peer.Port)
	}

	for _, peer := range peers {
		conn := connectToPeer(peer)
		dialedConns = append(dialedConns, conn)
	}
	go handleTransactions(dialedConns, peers)

	for {
	}
}
