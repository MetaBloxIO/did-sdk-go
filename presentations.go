package did

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"errors"
	"github.com/MetaBloxIO/did-sdk-go/registry"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
)

// create a presentation using 1 or more credentials. Currently unused
func CreatePresentation(credentials []VerifiableCredential, holderDocument DIDDocument, holderPrivKey *ecdsa.PrivateKey, nonce string) (*VerifiablePresentation, error) {
	presentationProof := CreateVPProof()
	presentationProof.Type = Secp256k1Sig
	presentationProof.VerificationMethod = holderDocument.Authentication
	presentationProof.JWSSignature = ""
	presentationProof.Created = time.Now().Format(time.RFC3339)
	presentationProof.ProofPurpose = "Authentication"
	presentationProof.Nonce = nonce
	presentationProof.PublicKeyString = crypto.FromECDSAPub(&holderPrivKey.PublicKey)
	context := []string{ContextSecp256k1, ContextCredential}
	presentationType := []string{"VerifiablePresentation"}
	presentation := NewPresentation(context, presentationType, credentials, holderDocument.ID, *presentationProof)
	//Create the proof's signature using a stringified version of the VP and the holder's private key.
	//This way, the signature can be verified by re-stringifying the VP and looking up the public key in the holder's DID document.
	//Verification will only succeed if the VP was unchanged since the signature and if the holder
	//public key matches the private key used to make the signature

	//This proof is only for the presentation itself; each credential also needs to be individually verified
	hashedVP := sha256.Sum256(ConvertVPToBytes(*presentation))

	signatureData, err := CreateJWSSignature(holderPrivKey, hashedVP[:])
	if err != nil {
		return nil, err
	}
	presentation.Proof.JWSSignature = signatureData
	return presentation, nil
}

// Verify a presentation. Need to first verify the presentation's proof using the holder's DID document.
// Afterwards, need to verify the proof of each credential included inside the presentation
func VerifyVP(presentation *VerifiablePresentation, registry *registry.Registry) (bool, error) {

	resolutionMeta, holderDoc, _ := Resolve(presentation.Holder, CreateResolutionOptions(), registry)
	if resolutionMeta.Error != "" {
		return false, errors.New(resolutionMeta.Error)
	}

	targetVM, err := holderDoc.RetrieveVerificationMethod(presentation.Proof.VerificationMethod)
	if err != nil {
		return false, err
	}

	holderKey, err := crypto.UnmarshalPubkey(presentation.Proof.PublicKeyString)
	if err != nil {
		return false, err
	}

	//currently only support EcdsaSecp256k1Signature2019, but it's possible we could introduce more
	var success bool
	switch presentation.Proof.Type {
	case Secp256k1Sig:
		if targetVM.MethodType != Secp256k1Key { //vm must be the same type as the proof
			return false, ErrSecp256k1WrongVMType
		}

		success = CompareAddresses(targetVM, holderKey) //vm must have the address that matches the proof's public key
		if !success {
			return false, ErrWrongAddress
		}

		success, err = VerifyVPSecp256k1(presentation, holderKey)
	default:
		return false, ErrUnknownProofType
	}

	if !success {
		return false, err
	}

	for _, credential := range presentation.VerifiableCredential { //verify each individual credential stored in the presentation
		success, err = VerifyVC(&credential, registry)
		if !success {
			return false, err
		}
	}

	return true, nil
}

// Verify that the provided public key matches the signature in the proof.
// Since we've made sure that the address in the holder vm matches this public key,
// verifying the signature here proves that the signature was made with the holder's private key
func VerifyVPSecp256k1(presentation *VerifiablePresentation, pubKey *ecdsa.PublicKey) (bool, error) {
	copiedVP := *presentation
	//have to make sure to remove the signature from the copy, as the original did not have a signature at the time the signature was generated
	copiedVP.Proof.JWSSignature = ""
	hashedVP := sha256.Sum256(ConvertVPToBytes(copiedVP))

	result, err := VerifyJWSSignature(presentation.Proof.JWSSignature, pubKey, hashedVP[:])
	if err != nil {
		return false, err
	}
	return result, nil
}

// convert presentation to bytes so it can be hashed
func ConvertVPToBytes(vp VerifiablePresentation) []byte {
	var convertedBytes []byte

	for _, item := range vp.Context {
		convertedBytes = bytes.Join([][]byte{convertedBytes, []byte(item)}, []byte{})
	}

	for _, item := range vp.Type {
		convertedBytes = bytes.Join([][]byte{convertedBytes, []byte(item)}, []byte{})
	}

	for _, item := range vp.VerifiableCredential {
		convertedBytes = bytes.Join([][]byte{convertedBytes, ConvertVCToBytes(item)}, []byte{})
	}

	convertedBytes = bytes.Join([][]byte{convertedBytes, []byte(vp.Holder), []byte(vp.Proof.Type), []byte(vp.Proof.Created), []byte(vp.Proof.VerificationMethod), []byte(vp.Proof.ProofPurpose), []byte(vp.Proof.JWSSignature), []byte(vp.Proof.Nonce), vp.Proof.PublicKeyString}, []byte{})
	return convertedBytes
}
