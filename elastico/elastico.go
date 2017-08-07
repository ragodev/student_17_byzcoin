package elastico


import (
	"gopkg.in/dedis/onet.v1"
	"github.com/dedis/cothority/byzcoin/blockchain"
	"fmt"
	"sync"
	"math/big"
	"gopkg.in/dedis/onet.v1/log"
	"crypto/sha256"
	"math/rand"
	"encoding/hex"
	"sort"
	"math"
)

const (
	// committee size
	c = 100
	// number of committee bits
	s = 2
	// number of committees
	comcnt
	target
	stateMining
	stateConsensus
)


type Elastico struct{
	// the node we represent
	*onet.TreeNodeInstance
	// nodes in the tree
	nodeList []*onet.TreeNode
	// node index in the tree
	index int
	// node state in the protocol
	state int
	// the global threshold of nodes needed computed from committee members count
	threshold int
	// all the members that this node has produced
	members map[string]*Member
	idm     sync.Mutex
	// the members in the directory committee
	directoryCommittee map[string]int
	dc sync.Mutex
	// number of CommitteeMembers messages that this node has received
	committeeMembersNo uint

	hashChan chan big.Int

	// prtocol start channel
	startProtocolChan chan startProtocolChan
	// channel to receive other nodes PoWs
	newMemberIDChan chan newMemberIDChan
	//channel to receive committee members from directory members
	committeeMembersChan chan committeeMembersChan

	// channels for pbft
	prePrepareChan chan prePrepareChan
	prepareChan    chan prepareChan
	commitChan     chan commitChan
	finishChan     chan finishChan
}




type Member struct {
	//id hash of this member
	idHashString string

	// members in this member's committee
	committeeMembers map[string]int

	// block that the committee wants to reach consensus
	committeeBlock *blockchain.TrBlock

	// if member is in a final committee
	isFinal bool
	// the random string that a final committee member generates
	randomString string
	// the random string set to pass to next round
	randomSet []string
	rs sync.Mutex

	// map of committee number to committee member id
	directory map[int](map[string]int)


	// directory messages that this member has received
	directoryMsgCnt int



	// the pbft of this NewMemberID
	tempPrePrepareMsg []*PrePrepare
	tempPrepareMsg []*Prepare
	tempCommitMsg  []*Commit
	// if the is the leader in the committee
	isLeaderchan chan bool
	// number of prepare messages received
	prepMsgCount int
	pmc          sync.Mutex
	// number of commit messages received
	commitMsgCount int
	cmc            sync.Mutex
	// thresholdPBFT which is 2.0/3.0 in pbft
	thresholdPBFT int
	// state that the member is in
	state int

	prePrepareChan chan bool
	prepareChan    chan *Prepare
	commitChan     chan *Commit
	finishChan     chan finishChan
}



func NewElastico(n *onet.TreeNodeInstance) (*Elastico, error){
	els := new(Elastico)
	els.TreeNodeInstance = n
	els.nodeList = n.Tree().List()
	els.index = -1
	els.committeeMembersNo = 0
	els.threshold = int (math.Ceil(float64(c) * 2.0 / 3.0))
	for i, tn := range els.nodeList{
		if tn.ID.Equal(n.TreeNode().ID){
			els.index = i
		}
	}
	if els.index == -1 {
		panic(fmt.Sprint("Could not find ourselves %+v in the list of nodes %+v", n, els.nodeList))
	}

	// register the channels so the protocol calls appropriate functions upon receiving messages
	if err := n.RegisterChannel(&els.startProtocolChan); err != nil {
		return els, err
	}
	if err := n.RegisterChannel(&els.newMemberIDChan); err != nil {
		return els, err
	}
	if err := n.RegisterChannel(&els.committeeMembersChan); err != nil{
		return els, err
	}
	if err := n.RegisterChannel(&els.prePrepareChan); err != nil{
		return els,err
	}
	if err := n.RegisterChannel(&els.prepareChan); err != nil{
		return els, err
	}
	if err := n.RegisterChannel(&els.commitChan); err != nil {
		return els,err
	}
	return els, nil
}


func (els *Elastico) Start() error{
	// broadcast to all nodes so that they start the protocol
	els.broadcast(func (tn *onet.TreeNode){
		if err := els.SendTo(tn, &StartProtocol{true}); err != nil {
			log.Error(els.Name(), "can't start protocol", tn.Name(), err)
			return err
		}
	})
	return nil
}


func (els *Elastico) Dispatch() error{
	for{
		select {
			case  <- els.startProtocolChan :
				els.handleStartProtocol()
			case msg := <- els.newMemberIDChan:
				els.handleNewMember(&msg.NewMemberID)
			case msg := <- els.committeeMembersChan :
				els.handleCommitteeMember(&msg.CommitteeMembers)
			case msg := <- els.prePrepareChan :
				els.handlePrePrepare(&msg.PrePrepare)
			case msg := <- els.prepareChan :
				els.handlePrepare(&msg.Prepare)
			case msg := <- els.commitChan:
				els.handleCommit(&msg.Commit)

		}
	}
}


func (els *Elastico) handleStartProtocol() error {
	els.state = stateMining
	// FIXME make the mining process more random
	for {
	var hashInt big.Int
		if els.state == stateMining {
			hashByte := els.computePoW()
			hashInt.SetBytes(hashByte)
			// FIXME make target comparable to big.Int
			if err := els.checkPoW(hashInt); err != nil {
				return err
			}
		}
	}
	return nil
}



func (els *Elastico) checkPoW(hashInt big.Int) error {
	if hashInt.Cmp(target) < 0 {
		hashHexString := hex.EncodeToString(hashInt.Bytes())
		els.dc.Lock()
		if len(els.directoryCommittee) < c {
			els.makeMember(hashHexString)
			els.directoryCommittee[hashHexString] = els.index
			els.broadcast(func(tn *onet.TreeNode) {
				if err := els.SendTo(tn, &NewMemberID{hashHexString, els.index}); err != nil {
					log.Error(els.Name(), "can't broadcast new member", tn.Name(), err)
					return err
				}
			})
		} else {
			els.makeMember(hashHexString)
			els.broadcastToDirectory(func(tn *onet.TreeNode) {
				if err := els.SendTo(tn, &NewMemberID{hashHexString, els.index}); err != nil {
					log.Error(els.Name(), "can't multicast new member to directory", tn.Name(), err)
					return err
				}
			})
		}
		els.dc.Unlock()
	}
	return nil
}


func (els *Elastico) computePoW() ([]byte) {
	h := sha256.New()
	h.Write([]byte(string(els.index + rand.Int())))
	hashByte := h.Sum(nil)
	return hashByte
}


func (els *Elastico) handleNewMember (newMember NewMemberID) error {
	els.dc.Lock()
	if len(els.directoryCommittee) < c {
		if newMember.NodeIndex != els.index {
			els.makeMember(newMember.HashHexString)
			els.directoryCommittee[newMember.HashHexString] = newMember.NodeIndex
		}
	}
	els.dc.Unlock()
	for hashString , index := range els.directoryCommittee {
		if els.index == index {
			return els.runAsDirectory(hashString, newMember)
		}
	}
	return nil
}


func (els *Elastico) runAsDirectory (hashString string, newMember NewMemberID) error {

	hashByte, err := hex.DecodeString(newMember.HashHexString)
	if err != nil {
		log.Error("mis-formatted hash string")
		return err
	}
	dirMember := els.members[hashString]
	committeeNo := getCommitteeNo(hashByte)

	if len(dirMember.directory[committeeNo]) < c {
		//FIXME add some validation before accepting the id
		committeeMap := dirMember.directory[committeeNo]
		committeeMap[newMember.HashHexString] = newMember.NodeIndex
	}

	return els.multicast(dirMember)
}


func (els *Elastico) multicast(dirMember *Member) error {

	completeCommittee := 0
	for i := 0 ; i < comcnt ; i++ {
		if len(dirMember.directory[i]) >= c {
			completeCommittee++
		}
	}


	if completeCommittee == comcnt {
		for committee, _:= range dirMember.directory{
			for member, node := range dirMember.directory[committee]{
				if err := els.SendTo(els.nodeList[node],
					&CommitteeMembers{
						dirMember.directory[committee],
						member}); err != nil{
					log.Error(els.Name(), "directory failed to member committee members", err)
					return err
				}
			}
		}
	}

	return nil
}


func (els *Elastico) handleCommitteeMember(committee CommitteeMembers) error {
	memberToUpdate := els.members[committee.DestMember]
	for coMember, node := range committee.CoMembers {
		if memberToUpdate.idHashString == coMember ||
			memberToUpdate.committeeMembers[coMember] != 0 {
			continue
		}
		memberToUpdate.committeeMembers[coMember] = node
	}
	go els.checkPbftState(memberToUpdate)
	return nil
}


func (els *Elastico) checkPbftState(member *Member) error {
	member.directoryMsgCnt++
	if member.directoryMsgCnt >= member.thresholdPBFT {
		if els.state == stateMining {
			els.state = stateConsensus
		}
		leader := selectLeader(member.committeeMembers)
		if leader == member.idHashString {
			member.isLeaderchan <- true
		}
		close(member.isLeaderchan)
		member.thresholdPBFT = int(math.Ceil(float64(len(member.committeeMembers))*2.0/3.0))
		member.state = pbftStatePrepare
		member.prePrepareChan <- true
	}
	return nil
}


func (els *Elastico) startPBFT(member *Member) {
	isLeader := <- member.isLeaderchan
	if isLeader {
		for comMember, node := range member.committeeMembers{
			if err := els.SendTo(els.nodeList[node], &PrePrepare{member.committeeBlock, comMember}); err != nil{
				log.Error(els.Name(), "could not start PBFT", els.nodeList[node], err)
				return
			}
		}
	}
}


func (els *Elastico) handlePrePrepare(prePrepare PrePrepare) error {
	member := els.members[prePrepare.DestMember]
	go els.handlePrepreparePBFT(member, prePrepare)
	return nil
}



func (els *Elastico) handlePrepreparePBFT(member *Member, prePrepare PrePrepare) {
	<-member.prePrepareChan
	member.state = pbftStatePrepare
	member.committeeBlock = prePrepare.TrBlock
	for coMember, node := range member.committeeMembers {
		if err := els.SendTo(els.nodeList[node], &Prepare{member.committeeBlock.HeaderHash, coMember}); err != nil {
			log.Error(els.Name(), "can't send prepare message", els.nodeList[node].Name(), err)
		}
	}
	for _, prepare := range member.tempPrepareMsg {
		member.prepareChan <- prepare
	}
	member.tempPrepareMsg = nil
}


func (els *Elastico) handlePrepare(prepare *Prepare) error {
	member := els.members[prepare.DestMember]
	if member.state == pbftStateNotReady {
		member.tempPrepareMsg = append(member.tempPrepareMsg, prepare)
		return nil
	} else if member.state == pbftStatePrepare {
		member.prepareChan <- prepare
	}
	return nil
}


func (els *Elastico) handlePreparePBFT(member *Member) {
	for {
		<- member.prepareChan
		member.prepMsgCount++
		if member.prepMsgCount >= member.thresholdPBFT {
			member.state = pbftStateCommit
			for coMember, node := range member.committeeMembers {
				if err := els.SendTo(els.nodeList[node], &Commit{member.committeeBlock.HeaderHash, coMember}); err != nil {
					log.Error(els.Name(), "can't send prepare message", els.nodeList[node].Name(), err)
				}
			}
			for _, commit := range member.tempCommitMsg {
				member.commitChan <- commit
			}
			member.commitChan = nil
			return
		}
	}
}

func (els *Elastico) handleCommit(commit *Commit) error{
	member := els.members[commit.DestMember]
	if member.state != pbftStateCommit {
		member.tempCommitMsg = append(member.tempCommitMsg, commit)
		return nil
	}
	member.commitChan <- commit
	return nil
}


func (els *Elastico) handleCommitPBFT(member *Member) {
	for {
		<- member.commitChan
		member.commitMsgCount++
		if member.commitMsgCount >= member.thresholdPBFT {
			member.state = pbftStateFinish
		}
	}
}


func selectLeader(committee map[string]int) string {
	var keys []string
	for member, _ := range committee{
		keys = append(keys, member)
	}
	sort.Strings(keys)
	return keys[0]
}


func getCommitteeNo(bytes []byte) int {
	hashInt := big.NewInt(bytes)
	toReturn := 0
	for i := 0; i < s ; i++{
		if hashInt.Bit(i) {
			toReturn += 1 << uint(i)
		}
	}
	return toReturn
}


func (els *Elastico) makeMember(hashHexString string) Member{
	member := new(Member)
	member.idHashString = hashHexString
	member.state = pbftStateNotReady
	go els.startPBFT(member)
	go els.handlePreparePBFT(member)
	go els.handleCommitPBFT(member)
	els.members[hashHexString] = member
	return member
}


func (els *Elastico) broadcast(sendCB func(node *onet.TreeNode)){
	for _,node := range els.nodeList{
		go sendCB(node)
	}
}

func (els *Elastico) broadcastToDirectory(sendCB func(node *onet.TreeNode)) {
	for _, node := range els.directoryCommittee{
		go sendCB(node)
	}
}

