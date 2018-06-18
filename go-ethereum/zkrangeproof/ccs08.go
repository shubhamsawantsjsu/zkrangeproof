// Copyright 2018 ING Bank N.V.
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

/*
This file contains the implementation of the ZKRP scheme proposed in the paper:
Efficient Protocols for Set Membership and Range Proofs
Jan Camenisch, Rafik Chaabouni, abhi shelat
Asiacrypt 2008
*/

package zkrangeproof

import (
	"errors"
	"strconv"
	"bytes"
	"math/big"
	"math"
	"crypto/rand"
	"github.com/ing-bank/zkrangeproof/go-ethereum/crypto/bn256"
)

//Constants that are going to be used frequently, then we just need to compute them once.
var (
	G1 = new(bn256.G1).ScalarBaseMult(new(big.Int).SetInt64(1))
	G2 = new(bn256.G2).ScalarBaseMult(new(big.Int).SetInt64(1))
	E = bn256.Pair(G1, G2)
)

/*
paramsSet contains elements generated by the verifier, which are necessary for the prover.
This must be computed in a trusted setup.
*/
type paramsSet struct {
	signatures map[int64]*bn256.G2
	H *bn256.G2
	// TODO:must protect the private key
	kp keypair
	// u determines the amount of signatures we need in the public params. 
	// Each signature can be compressed to just 1 field element of 256 bits.
	// Then the parameters have minimum size equal to 256*u bits. 
	// l determines how many pairings we need to compute, then in order to improve
	// the verifiers performance we want to minize it.
	// Namely, we have 2*l pairings for the prover and 3*l for the verifier.  
}

/*
paramsUL contains elements generated by the verifier, which are necessary for the prover.
This must be computed in a trusted setup.
*/
type paramsUL struct {
	signatures map[string]*bn256.G2
	H *bn256.G2
	// TODO:must protect the private key
	kp keypair
	// u determines the amount of signatures we need in the public params. 
	// Each signature can be compressed to just 1 field element of 256 bits.
	// Then the parameters have minimum size equal to 256*u bits. 
	// l determines how many pairings we need to compute, then in order to improve
	// the verifiers performance we want to minize it.
	// Namely, we have 2*l pairings for the prover and 3*l for the verifier.  
	u,l int64
}

/*
proofSet contains the necessary elements for the ZK Set Membership proof.
*/
type proofSet struct {
	V *bn256.G2
	D,C *bn256.G2
	a *bn256.GT
	s,t,zsig,zv *big.Int
	c,m,zr *big.Int
}

/*
proofUL contains the necessary elements for the ZK proof.
*/
type proofUL struct {
	V []*bn256.G2
	D,C *bn256.G2
	a []*bn256.GT
	s,t,zsig,zv []*big.Int
	c,m,zr *big.Int
}

/*
SetupSet generates the signature for the elements in the set.
*/
func SetupSet(s []int64) (paramsSet, error) {
	var (
		i int
		p paramsSet
	)
	p.kp, _ = keygen()

	p.signatures = make(map[int64]*bn256.G2)
	for i=0; i < len(s); i++ {
		sig_i, _ := sign(new(big.Int).SetInt64(int64(s[i])), p.kp.privk)
		p.signatures[s[i]] = sig_i 
	}
	//TODO: protect the 'master' key
	h := GetBigInt("18560948149108576432482904553159745978835170526553990798435819795989606410925")
	p.H = new(bn256.G2).ScalarBaseMult(h)
	return p, nil
}

/*
SetupUL generates the signature for the interval [0,u^l).
The value of u should be roughly b/log(b), but we can choose smaller values in
order to get smaller parameters, at the cost of having worse performance.
*/
func SetupUL(u, l int64) (paramsUL, error) {
	var (
		i int64
		p paramsUL
	)
	p.kp, _ = keygen()

	p.signatures = make(map[string]*bn256.G2)
	for i=0; i < u; i++ {
		sig_i, _ := sign(new(big.Int).SetInt64(i), p.kp.privk)
		p.signatures[strconv.FormatInt(i, 10)] = sig_i 
	}
	//TODO: protect the 'master' key
	h := GetBigInt("18560948149108576432482904553159745978835170526553990798435819795989606410925")
	p.H = new(bn256.G2).ScalarBaseMult(h)
	p.u = u
	p.l = l
	return p, nil
}

/*
ProveSet method is used to produce the ZK Set Membership proof.
*/
func ProveSet(x int64, r *big.Int, p paramsSet) (proofSet, error) {
	var (
		v *big.Int
		proof_out proofSet
	)

	// Initialize variables
	proof_out.D = new(bn256.G2) 
	proof_out.D.SetInfinity()
	proof_out.m, _ = rand.Int(rand.Reader, bn256.Order)
	
	D := new(bn256.G2)
	v, _ = rand.Int(rand.Reader, bn256.Order)
	A, ok := p.signatures[x]
	if ok {
		// D = g^s.H^m
		D = new(bn256.G2).ScalarMult(p.H, proof_out.m)
		proof_out.s, _ = rand.Int(rand.Reader, bn256.Order)
		aux := new(bn256.G2).ScalarBaseMult(proof_out.s)
		D.Add(D, aux)

		proof_out.V = new(bn256.G2).ScalarMult(A, v)
		proof_out.t, _ = rand.Int(rand.Reader, bn256.Order)
		proof_out.a = bn256.Pair(G1, proof_out.V)
		proof_out.a.ScalarMult(proof_out.a, proof_out.s)
		proof_out.a.Invert(proof_out.a)
		proof_out.a.Add(proof_out.a, new(bn256.GT).ScalarMult(E, proof_out.t))
	} else {
		return proof_out, errors.New("Could not generate proof. Element does not belong to the interval.")
	}
	proof_out.D.Add(proof_out.D, D)
	
	proof_out.C, _ = CommitSet(new(big.Int).SetInt64(x), r, p)
	proof_out.c, _ = HashSet(proof_out.a, proof_out.D)
	proof_out.c = Mod(proof_out.c, bn256.Order)

	proof_out.zr = Sub(proof_out.m, Multiply(r, proof_out.c))
	proof_out.zr = Mod(proof_out.zr, bn256.Order)
	proof_out.zsig = Sub(proof_out.s, Multiply(new(big.Int).SetInt64(x), proof_out.c))
	proof_out.zsig = Mod(proof_out.zsig, bn256.Order)
	proof_out.zv = Sub(proof_out.t, Multiply(v, proof_out.c))
	proof_out.zv = Mod(proof_out.zv, bn256.Order)
	return proof_out, nil
}

/*
ProveUL method is used to produce the ZKRP proof.
*/
func ProveUL(x,r *big.Int, p paramsUL) (proofUL, error) {
	var (
		i int64
		v []*big.Int
		proof_out proofUL
	)
	decx, _ := Decompose(x, p.u, p.l)	

	// Initialize variables
	v = make([]*big.Int, p.l, p.l)
	proof_out.V  = make([]*bn256.G2, p.l, p.l)
	proof_out.a  = make([]*bn256.GT, p.l, p.l)
	proof_out.s = make([]*big.Int, p.l, p.l)
	proof_out.t = make([]*big.Int, p.l, p.l)
	proof_out.zsig = make([]*big.Int, p.l, p.l)
	proof_out.zv = make([]*big.Int, p.l, p.l)
	proof_out.D = new(bn256.G2) 
	proof_out.D.SetInfinity()
	proof_out.m, _ = rand.Int(rand.Reader, bn256.Order)
	
	// D = H^m
	D := new(bn256.G2).ScalarMult(p.H, proof_out.m)
	for i = 0; i< p.l; i++ {
		v[i], _ = rand.Int(rand.Reader, bn256.Order)
		A, ok := p.signatures[strconv.FormatInt(decx[i], 10)]
		if ok {
			proof_out.V[i] = new(bn256.G2).ScalarMult(A, v[i])
			proof_out.s[i], _ = rand.Int(rand.Reader, bn256.Order)
			proof_out.t[i], _ = rand.Int(rand.Reader, bn256.Order)
			proof_out.a[i] = bn256.Pair(G1, proof_out.V[i])
			proof_out.a[i].ScalarMult(proof_out.a[i], proof_out.s[i])
			proof_out.a[i].Invert(proof_out.a[i])
			proof_out.a[i].Add(proof_out.a[i], new(bn256.GT).ScalarMult(E, proof_out.t[i]))

			ui := new(big.Int).Exp(new(big.Int).SetInt64(p.u), new(big.Int).SetInt64(i), nil)
			muisi := new(big.Int).Mul(proof_out.s[i], ui)
			muisi = Mod(muisi, bn256.Order)
			aux := new(bn256.G2).ScalarBaseMult(muisi)
			D.Add(D, aux)
		} else {
			return proof_out, errors.New("Could not generate proof. Element does not belong to the interval.")
		}
	}	
	proof_out.D.Add(proof_out.D, D)
	
	proof_out.C, _ = Commit(x, r, p)
	proof_out.c, _ = Hash(proof_out.a, proof_out.D)
	proof_out.c = Mod(proof_out.c, bn256.Order)

	proof_out.zr = Sub(proof_out.m, Multiply(r, proof_out.c))
	proof_out.zr = Mod(proof_out.zr, bn256.Order)
	for i = 0; i< p.l; i++ {
		proof_out.zsig[i] = Sub(proof_out.s[i], Multiply(new(big.Int).SetInt64(decx[i]), proof_out.c))
		proof_out.zsig[i] = Mod(proof_out.zsig[i], bn256.Order)
		proof_out.zv[i] = Sub(proof_out.t[i], Multiply(v[i], proof_out.c))
		proof_out.zv[i] = Mod(proof_out.zv[i], bn256.Order)
	}
	return proof_out, nil
}

/*
VerifySet is used to validate the ZK Set Membership proof. It returns true iff the proof is valid.
*/
func VerifySet(proof_out *proofSet, p *paramsSet) (bool, error) {
	var (
		D *bn256.G2
		r1, r2 bool
		p1,p2 *bn256.GT
	)
	// D == C^c.h^ zr.g^zsig ?
	D = new(bn256.G2).ScalarMult(proof_out.C, proof_out.c)
	D.Add(D, new(bn256.G2).ScalarMult(p.H, proof_out.zr)) 	
	aux := new(bn256.G2).ScalarBaseMult(proof_out.zsig)
	D.Add(D, aux) 	

	DBytes := D.Marshal()
	pDBytes := proof_out.D.Marshal()
	r1 = bytes.Equal(DBytes, pDBytes)

	r2 = true
	// a == [e(V,y)^c].[e(V,g)^-zsig].[e(g,g)^zv]
	p1 = bn256.Pair(p.kp.pubk, proof_out.V)
	p1.ScalarMult(p1, proof_out.c)
	p2 = bn256.Pair(G1, proof_out.V)
	p2.ScalarMult(p2, proof_out.zsig)
	p2.Invert(p2)
	p1.Add(p1, p2)
	p1.Add(p1, new(bn256.GT).ScalarMult(E, proof_out.zv))

	pBytes := p1.Marshal()
	aBytes := proof_out.a.Marshal()
	r2 = r2 && bytes.Equal(pBytes, aBytes) 
	return r1 && r2, nil
}

/*
VerifyUL is used to validate the ZKRP proof. It returns true iff the proof is valid.
*/
func VerifyUL(proof_out *proofUL, p *paramsUL) (bool, error) {
	var (
		i int64
		D *bn256.G2
		r1, r2 bool
		p1,p2 *bn256.GT
	)
	// D == C^c.h^ zr.g^zsig ?
	D = new(bn256.G2).ScalarMult(proof_out.C, proof_out.c)
	D.Add(D, new(bn256.G2).ScalarMult(p.H, proof_out.zr)) 	
	for i = 0; i< p.l; i++ {
		ui := new(big.Int).Exp(new(big.Int).SetInt64(p.u), new(big.Int).SetInt64(i), nil)
		muizsigi := new(big.Int).Mul(proof_out.zsig[i], ui)
		muizsigi = Mod(muizsigi, bn256.Order)
		aux := new(bn256.G2).ScalarBaseMult(muizsigi)
		D.Add(D, aux) 	
	}

	DBytes := D.Marshal()
	pDBytes := proof_out.D.Marshal()
	r1 = bytes.Equal(DBytes, pDBytes)

	r2 = true
	for i = 0; i < p.l; i++ {
		// a == [e(V,y)^c].[e(V,g)^-zsig].[e(g,g)^zv]
		p1 = bn256.Pair(p.kp.pubk, proof_out.V[i])
		p1.ScalarMult(p1, proof_out.c)
		p2 = bn256.Pair(G1, proof_out.V[i])
		p2.ScalarMult(p2, proof_out.zsig[i])
		p2.Invert(p2)
		p1.Add(p1, p2)
		p1.Add(p1, new(bn256.GT).ScalarMult(E, proof_out.zv[i]))
	
		pBytes := p1.Marshal()
		aBytes := proof_out.a[i].Marshal()
		r2 = r2 && bytes.Equal(pBytes, aBytes) 
	}
	return r1 && r2, nil
}

/*
proof contains the necessary elements for the ZK proof.
*/
type proof struct {
	p1,p2 proofUL
}

/*
params contains elements generated by the verifier, which are necessary for the prover.
This must be computed in a trusted setup.
*/
type params struct {
	p *paramsUL 
	a,b int64
}

type ccs08 struct {
	p *params
	x, r *big.Int
	proof_out proof
	pubk *bn256.G1
}

/*
Setup receives integers a and b, and configures the parameters for the rangeproof scheme.
*/
func (zkrp *ccs08) Setup(a,b int64) (error) {
	// Compute optimal values for u and l
	var (
		u,l int64
		logb float64
		p *params
	)
	if a > b {
		zkrp.p = nil
		return errors.New("a must be less than or equal to b")
	}
	p = new(params)
	logb = math.Log(float64(b))
	if logb != 0 {
		// TODO: understand how to find optimal parameters
		//u = b / int64(logb)
		u = 100
		if u != 0 {
			l = 0
			for i:=b; i>0; i=i/u {
				l=l+1
			}
			params_out, e := SetupUL(u, l)
			p.p = &params_out
			p.a = a
			p.b = b
			zkrp.p = p
			return e
		} else {
			zkrp.p = nil
			return errors.New("u is zero") 
		}
	} else {
		zkrp.p = nil
		return errors.New("log(b) is zero") 
	}
}

/*
Prove method is responsible for generating the zero knowledge proof.
*/
func (zkrp *ccs08) Prove() (error) {
	ul := new(big.Int).Exp(new(big.Int).SetInt64(zkrp.p.p.u), new(big.Int).SetInt64(zkrp.p.p.l), nil)
	
	// x - b + ul
	xb := new(big.Int).Sub(zkrp.x, new(big.Int).SetInt64(zkrp.p.b))
	xb.Add(xb, ul)
	first, _ := ProveUL(xb, zkrp.r, *zkrp.p.p)

	// x - a
	xa := new(big.Int).Sub(zkrp.x, new(big.Int).SetInt64(zkrp.p.a))
	second, _ := ProveUL(xa, zkrp.r, *zkrp.p.p)

	zkrp.proof_out.p1 = first
	zkrp.proof_out.p2 = second
	return nil
}

/*
Verify is responsible for validating the proof.
*/
func (zkrp *ccs08) Verify() (bool, error) {
	first, _ := VerifyUL(&zkrp.proof_out.p1, zkrp.p.p)
	second, _ := VerifyUL(&zkrp.proof_out.p2, zkrp.p.p)
	return first && second, nil
}

