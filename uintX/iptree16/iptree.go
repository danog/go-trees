// Package iptree16 implements radix tree data structure for IPv4 and IPv6 networks as key and uint16 as value.
package iptree16

// !!!DON'T EDIT!!! Generated by infobloxopen/go-trees/etc from <name>tree{{.bits}} with etc -s uint16 -d uintX.yaml -t ./<name>tree\{\{.bits\}\}

import "net"

const (
	iPv4Bits = net.IPv4len * 8
	iPv6Bits = net.IPv6len * 8
)

var (
	iPv4MaxMask = net.CIDRMask(iPv4Bits, iPv4Bits)
	iPv6MaxMask = net.CIDRMask(iPv6Bits, iPv6Bits)
)

// Tree is a radix tree for IPv4 and IPv6 networks.
type Tree struct {
	root32 *node32
	root64 *node64s
}

// Pair represents a key-value pair returned by Enumerate method.
type Pair struct {
	Key   *net.IPNet
	Value uint16
}

type subTree64 *node64

// NewTree creates empty tree.
func NewTree() *Tree {
	return &Tree{}
}

// InsertNet inserts value using given network as a key. The method returns new tree (old one remains unaffected).
func (t *Tree) InsertNet(n *net.IPNet, value uint16) *Tree {
	if n == nil {
		return t
	}

	var (
		r32 *node32
		r64 *node64s
	)

	if t != nil {
		r32 = t.root32
		r64 = t.root64
	}

	if key, bits := iPv4NetToUint32(n); bits >= 0 {
		return &Tree{
			root32: r32.Insert(key, bits, value),
			root64: r64,
		}
	}

	if MSKey, MSBits, LSKey, LSBits := iPv6NetToUint64Pair(n); MSBits >= 0 {
		var r *node64
		if v, ok := r64.ExactMatch(MSKey, MSBits); ok {
			r = v
		}

		return &Tree{
			root32: r32,
			root64: r64.Insert(MSKey, MSBits, r.Insert(LSKey, LSBits, value)),
		}
	}

	return t
}

// InplaceInsertNet inserts (or replaces) value using given network as a key in current tree.
func (t *Tree) InplaceInsertNet(n *net.IPNet, value uint16) {
	if n == nil {
		return
	}

	if key, bits := iPv4NetToUint32(n); bits >= 0 {
		t.root32 = t.root32.InplaceInsert(key, bits, value)
	} else if MSKey, MSBits, LSKey, LSBits := iPv6NetToUint64Pair(n); MSBits >= 0 {
		var r *node64
		if v, ok := t.root64.ExactMatch(MSKey, MSBits); ok {
			r = v.InplaceInsert(LSKey, LSBits, value)
			if r != v {
				t.root64 = t.root64.InplaceInsert(MSKey, MSBits, r)
			}
		} else {
			t.root64 = t.root64.InplaceInsert(MSKey, MSBits, r.InplaceInsert(LSKey, LSBits, value))
		}
	}
}

// InsertIP inserts value using given IP address as a key. The method returns new tree (old one remains unaffected).
func (t *Tree) InsertIP(ip net.IP, value uint16) *Tree {
	return t.InsertNet(newIPNetFromIP(ip), value)
}

// InplaceInsertIP inserts (or replaces) value using given IP address as a key in current tree.
func (t *Tree) InplaceInsertIP(ip net.IP, value uint16) {
	t.InplaceInsertNet(newIPNetFromIP(ip), value)
}

// Enumerate returns channel which is populated by key-value pairs of tree content.
func (t *Tree) Enumerate() chan Pair {
	ch := make(chan Pair)

	go func() {
		defer close(ch)

		if t == nil {
			return
		}

		t.enumerate(ch)
	}()

	return ch
}

// GetByNet gets value for network which is equal to or contains given network.
func (t *Tree) GetByNet(n *net.IPNet) (uint16, bool) {
	if t == nil || n == nil {
		return 0, false
	}

	if key, bits := iPv4NetToUint32(n); bits >= 0 {
		return t.root32.Match(key, bits)
	}

	if MSKey, MSBits, LSKey, LSBits := iPv6NetToUint64Pair(n); MSBits >= 0 {
		s, ok := t.root64.Match(MSKey, MSBits)
		if !ok {
			return 0, false
		}

		v, ok := s.Match(LSKey, LSBits)
		if ok || MSBits < key64BitSize {
			return v, ok
		}

		s, ok = t.root64.Match(MSKey, MSBits-1)
		if !ok {
			return 0, false
		}

		return s.Match(LSKey, LSBits)
	}

	return 0, false
}

// GetByIP gets value for network which is equal to or contains given IP address.
func (t *Tree) GetByIP(ip net.IP) (uint16, bool) {
	return t.GetByNet(newIPNetFromIP(ip))
}

// DeleteByNet removes subtree which is contained by given network. The method returns new tree (old one remains unaffected) and flag indicating if deletion happens indeed.
func (t *Tree) DeleteByNet(n *net.IPNet) (*Tree, bool) {
	if t == nil || n == nil {
		return t, false
	}

	if key, bits := iPv4NetToUint32(n); bits >= 0 {
		r, ok := t.root32.Delete(key, bits)
		if ok {
			return &Tree{root32: r, root64: t.root64}, true
		}
	} else if MSKey, MSBits, LSKey, LSBits := iPv6NetToUint64Pair(n); MSBits >= 0 {
		if v, ok := t.root64.ExactMatch(MSKey, MSBits); ok {
			r, ok := v.Delete(LSKey, LSBits)
			if ok {
				r64 := t.root64
				if r == nil {
					r64, _ = r64.Delete(MSKey, MSBits)
				} else {
					r64 = r64.Insert(MSKey, MSBits, r)
				}

				return &Tree{root32: t.root32, root64: r64}, true
			}
		}
	}

	return t, false
}

// DeleteByIP removes node by given IP address. The method returns new tree (old one remains unaffected) and flag indicating if deletion happens indeed.
func (t *Tree) DeleteByIP(ip net.IP) (*Tree, bool) {
	return t.DeleteByNet(newIPNetFromIP(ip))
}

func (t *Tree) enumerate(ch chan Pair) {
	for n := range t.root32.Enumerate() {
		mask := net.CIDRMask(int(n.bits), iPv4Bits)
		ch <- Pair{
			Key: &net.IPNet{
				IP:   unpackUint32ToIP(n.key).Mask(mask),
				Mask: mask},
			Value: n.value}
	}

	for n := range t.root64.Enumerate() {
		MSIP := append(unpackUint64ToIP(n.key), make(net.IP, 8)...)
		for m := range n.value.Enumerate() {
			LSIP := unpackUint64ToIP(m.key)
			mask := net.CIDRMask(int(n.bits+m.bits), iPv6Bits)
			ch <- Pair{
				Key: &net.IPNet{
					IP:   append(MSIP[0:8], LSIP...).Mask(mask),
					Mask: mask},
				Value: m.value}
		}
	}
}

func iPv4NetToUint32(n *net.IPNet) (uint32, int) {
	ip := n.IP.To4()
	if ip == nil {
		return 0, -1
	}

	ones, bits := n.Mask.Size()
	if bits != iPv4Bits {
		return 0, -1
	}

	return packIPToUint32(ip), ones
}

func packIPToUint32(x net.IP) uint32 {
	return (uint32(x[0]) << 24) | (uint32(x[1]) << 16) | (uint32(x[2]) << 8) | uint32(x[3])
}

func unpackUint32ToIP(x uint32) net.IP {
	return net.IP{byte(x >> 24 & 0xff), byte(x >> 16 & 0xff), byte(x >> 8 & 0xff), byte(x & 0xff)}
}

func iPv6NetToUint64Pair(n *net.IPNet) (uint64, int, uint64, int) {
	if len(n.IP) != net.IPv6len {
		return 0, -1, 0, -1
	}

	ones, bits := n.Mask.Size()
	if bits != iPv6Bits {
		return 0, -1, 0, -1
	}

	MSBits := key64BitSize
	LSBits := 0
	if ones > key64BitSize {
		LSBits = ones - key64BitSize
	} else {
		MSBits = ones
	}

	return packIPToUint64(n.IP), MSBits, packIPToUint64(n.IP[8:]), LSBits
}

func packIPToUint64(x net.IP) uint64 {
	return (uint64(x[0]) << 56) | (uint64(x[1]) << 48) | (uint64(x[2]) << 40) | (uint64(x[3]) << 32) |
		(uint64(x[4]) << 24) | (uint64(x[5]) << 16) | (uint64(x[6]) << 8) | uint64(x[7])
}

func unpackUint64ToIP(x uint64) net.IP {
	return net.IP{
		byte(x >> 56 & 0xff),
		byte(x >> 48 & 0xff),
		byte(x >> 40 & 0xff),
		byte(x >> 32 & 0xff),
		byte(x >> 24 & 0xff),
		byte(x >> 16 & 0xff),
		byte(x >> 8 & 0xff),
		byte(x & 0xff)}
}

func newIPNetFromIP(ip net.IP) *net.IPNet {
	if ip4 := ip.To4(); ip4 != nil {
		return &net.IPNet{IP: ip4, Mask: iPv4MaxMask}
	}

	if ip6 := ip.To16(); ip6 != nil {
		return &net.IPNet{IP: ip6, Mask: iPv6MaxMask}
	}

	return nil
}
