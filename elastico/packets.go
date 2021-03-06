package elastico

import (
"github.com/dedis/cothority/byzcoin/blockchain"
"gopkg.in/dedis/onet.v1"
	"gopkg.in/dedis/onet.v1/network"
	//"math/big"
)

const (
	pbftStateNotReady = iota
	pbftStatePrePrepare
	pbftStatePrepare
	pbftStateCommit
	pbftStateTransit
	pbftStatePrePrepareFinal
	pbftStatePrepareFinal
	pbftStateCommitFinal
	pbftStateFinish
)

func init() {
	for _, i := range []interface{}{
		StartProtocol{},
		NewMember{},
		CommitteeMembers{},
		PrePrepare{},
		PrePrepareFinal{},
		Prepare{},
		PrepareFinal{},
		Commit{},
		CommitFinal{},
		Finish{},
		BlockToFinalCommittee{},
	} {
		network.RegisterMessage(i)
	}
}

type startProtocolChan struct{
	*onet.TreeNode
	StartProtocol
}

type StartProtocol struct{
	Block          *blockchain.TrBlock
	CommitteeCount int
	CommitteeSize  int
	FinalCommittee int
	Target         int
}

type readyChan struct {
	*onet.TreeNode
	Ready
}

type Ready struct {

}

type pbftStartChan struct {
	*onet.TreeNode
	PBFTStart
}

type PBFTStart struct {

}

type miningChan struct{
	*onet.TreeNode
	Mining
}

type Mining struct{

}

type NewMember struct {
	HashHexString string
	NodeIndex     int
}

type newMemberChan struct{
	*onet.TreeNode
	NewMember
}

type CommitteeMembers struct{
	CoMembers  map[string]int
	FinMembers map[string]int
	DestMember string
	CommitteeNo int
}

type committeeMembersChan struct{
	*onet.TreeNode
	CommitteeMembers
}

type PrePrepare struct {
	*blockchain.TrBlock
	DestMember string
}

type prePrepareChan struct {
	*onet.TreeNode
	PrePrepare
}

type PrePrepareFinal struct{
	HeaderHash string
	DestMember string
}

type prePrepareFinalChan struct{
	*onet.TreeNode
	PrePrepareFinal
}

type Prepare struct {
	HeaderHash string
	DestMember string
}

type prepareChan struct {
	*onet.TreeNode
	Prepare
}

type PrepareFinal struct {
	HedearHash string
	DestMember string
}

type prepareFinalChan struct {
	*onet.TreeNode
	PrepareFinal
}

type Commit struct {
	HeaderHash string
	DestMember string
}

type commitChan struct {
	*onet.TreeNode
	Commit
}

type CommitFinal struct {
	HeaderHash string
	DestMember string
}

type commitFinalChan struct {
	*onet.TreeNode
	CommitFinal
}

type Finish struct {}

type FinishChan struct {
	*onet.TreeNode
	Finish
}

type FinishRoot struct{}

type finishRootChan struct {
	*onet.TreeNode
	FinishRoot
}

type BlockToFinalCommittee struct {
	HeaderHash  string
	DestMember  string
	CommitteeNo int
}

type blockToFinalCommitteeChan struct {
	*onet.TreeNode
	BlockToFinalCommittee
}


