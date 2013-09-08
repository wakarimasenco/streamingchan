package node

type NodeInfoList []NodeInfo

func (this NodeInfoList) Len() int {
	return len(this)
}
func (this NodeInfoList) Less(i, j int) bool {
	return this[i].NodeId > this[j].NodeId
}
func (this NodeInfoList) Swap(i, j int) {
	this[i], this[j] = this[j], this[i]
}
