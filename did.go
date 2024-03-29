package did

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"github.com/MetaBloxIO/did-sdk-go/registry"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/mr-tron/base58"
)

//DID is created by taking a public key, taking its Keccak256 hash, and encoding the hash using base58. We then add 'did:metablox:' to the front

func GenerateDIDString(privKey *ecdsa.PrivateKey) string {
	pubData := crypto.FromECDSAPub(&privKey.PublicKey)

	hash := crypto.Keccak256(pubData)
	didString := base58.Encode(hash)
	returnString := "did:metablox:" + didString
	return returnString
}

// TODO: check that this function can be safely removed. The foundation service doesn't need to create new DID documents; however, some other system may want to import this function
func CreateDID(privKey *ecdsa.PrivateKey) *DIDDocument {

	document := new(DIDDocument)

	document.ID = GenerateDIDString(privKey)
	document.Context = make([]string, 0)
	document.Context = append(document.Context, ContextSecp256k1)
	document.Context = append(document.Context, ContextDID)
	document.Created = time.Now().Format(time.RFC3339)
	document.Updated = document.Created
	document.Version = 1

	address := crypto.PubkeyToAddress(privKey.PublicKey)

	VM := VerificationMethod{}
	VM.ID = document.ID + "#verification"
	VM.BlockchainAccountId = "eip155:" + issuerChainId.String() + ":" + address.Hex()
	VM.Controller = document.ID
	VM.MethodType = Secp256k1Key

	document.VerificationMethod = append(document.VerificationMethod, VM)
	document.Authentication = VM.ID

	return document
}

// TODO: check that this function can be safely removed
func DocumentToJson(document *DIDDocument) ([]byte, error) {
	jsonDoc, err := json.Marshal(document)
	if err != nil {
		return nil, err
	}
	return jsonDoc, nil
}

// TODO: check that this function can be safely removed
func JsonToDocument(jsonDoc []byte) (*DIDDocument, error) {
	document := CreateDIDDocument()
	err := json.Unmarshal(jsonDoc, document)
	if err != nil {
		return nil, err
	}
	return document, nil
}

// check format of DID string
func IsDIDValid(did []string) bool {
	if len(did) != 3 {
		fmt.Println("Not exactly 3 sections in DID")
		return false
	}
	prefix := did[0]
	if prefix != "did" {
		fmt.Println("First section of DID was '" + prefix + "' instead of 'did'")
		return false
	}

	methodName := did[1]
	if methodName != "metablox" {
		fmt.Println("Second section of DID was '" + methodName + "'instead of 'metablox'")
		return false
	}

	identifierPattern := `^([a-zA-Z0-9\._\-]*(%[0-9A-Fa-f][0-9A-Fa-f])*)*$`
	identifierExp, _ := regexp.Compile(identifierPattern)

	identifierSection := did[2]
	match := identifierExp.MatchString(identifierSection)
	if !match {
		fmt.Println("Identifier section is formatted incorrectly")
		return false
	}

	if len(identifierSection) == 0 {
		fmt.Println("Identifier is empty")
		return false
	}

	return true
}

// split did string into 3 sections. First two should be 'did' and 'metablox', last one wil be the identifier
func SplitDIDString(did string) []string {
	return strings.Split(did, ":")
}

// splits did and checks that it is formatted correctly
func PrepareDID(did string) ([]string, bool) {
	splitString := SplitDIDString(did)
	valid := IsDIDValid(splitString)
	return splitString, valid
}

func GetDocument(targetDID string, registry *registry.Registry) (*DIDDocument, [32]byte, error) {
	address, err := registry.Dids(nil, targetDID)
	if err != nil {
		return nil, [32]byte{0}, err
	}

	document := new(DIDDocument)

	document.ID = "did:metablox:" + targetDID
	document.Context = make([]string, 0)
	document.Context = append(document.Context, ContextSecp256k1)
	document.Context = append(document.Context, ContextDID)
	document.Created = time.Now().Format(time.RFC3339) //todo: need to get this from contract
	document.Updated = document.Created                //todo: need to get this from contract
	document.Version = 1                               //todo: need to get this from contract

	VM := VerificationMethod{}
	VM.ID = document.ID + "#verification"
	VM.BlockchainAccountId = "eip155:" + issuerChainId.String() + ":" + address.Hex()
	VM.Controller = document.ID
	VM.MethodType = Secp256k1Key

	document.VerificationMethod = append(document.VerificationMethod, VM)
	document.Authentication = VM.ID

	placeholderHash := [32]byte{94, 241, 27, 134, 190, 223, 112, 91, 189, 49, 221, 31, 228, 35, 189, 213, 251, 60, 60, 210, 162, 45, 151, 3, 31, 78, 41, 239, 41, 75, 198, 139}
	return document, placeholderHash, nil
}

// generate the did document that matches the provided did string. Any errors are returned in the ResolutionMetadata.
// Note that options currently does nothing; including it is a requirement according to W3C specifications, but we don't do anything with it right now
func Resolve(did string, options *ResolutionOptions, registry *registry.Registry) (*ResolutionMetadata, *DIDDocument, *DocumentMetadata) {
	splitDID, valid := PrepareDID(did)
	if !valid {
		return &ResolutionMetadata{Error: "invalid Did"}, nil, &DocumentMetadata{}
	}

	generatedDocument, _, err := GetDocument(splitDID[2], registry)
	if err != nil {
		return &ResolutionMetadata{Error: err.Error()}, nil, nil
	}

	docID, success := PrepareDID(generatedDocument.ID)
	if !success {
		return &ResolutionMetadata{Error: "document DID is invalid"}, nil, nil
	}

	if docID[2] != splitDID[2] { //identifier of the document should match provided did
		return &ResolutionMetadata{Error: "generated document DID does not match provided DID"}, nil, nil
	}

	//compare document hash to the hash value given by contract.GetDocument() to ensure data integrity

	/*comparisonHash := sha256.Sum256(ConvertDocToBytes(*generatedDocument))	//disabling this at the moment to avoid needing to update placeholderHash while we're still modfiying document layout
	if comparisonHash != generatedHash {
		return &ResolutionMetadata{Error: "document failed hash check"}, nil, nil
	}*/
	return &ResolutionMetadata{}, generatedDocument, nil
}

// generate a did document and return it in a specific data format (currently just JSON)
func ResolveRepresentation(did string, options *RepresentationResolutionOptions, registry *registry.Registry) (*RepresentationResolutionMetadata, []byte, *DocumentMetadata) {
	//Should be similar to Resolve, but returns the document in a specific representation format.
	//Representation type is included in options and returned in resolution metadata
	readOptions := CreateResolutionOptions()
	readResolutionMeta, document, readDocumentMeta := Resolve(did, readOptions, registry)
	if readResolutionMeta.Error != "" {
		return &RepresentationResolutionMetadata{Error: readResolutionMeta.Error}, nil, nil
	}

	switch options.Accept {
	case "application/did+json":
		fallthrough
	default: //default to JSON format if options.Accept is empty/invalid
		byteStream, err := json.Marshal(document)
		if err != nil {
			return &RepresentationResolutionMetadata{Error: "failed to convert document into JSON"}, nil, nil
		}
		return &RepresentationResolutionMetadata{ContentType: "application/did+json"}, byteStream, readDocumentMeta
	}
}

// convert document into byte array so it can be hashed (appears to be unused currently)
func ConvertDocToBytes(doc DIDDocument) []byte {
	var convertedBytes []byte

	sort.SliceStable(doc.Context, func(i, j int) bool { //have to sort arrays alphabetically before iterating over them to ensure a consistent ordering
		return doc.Context[i] < doc.Context[j]
	})
	for _, item := range doc.Context {
		convertedBytes = bytes.Join([][]byte{convertedBytes, []byte(item)}, []byte{})
	}

	convertedBytes = bytes.Join([][]byte{convertedBytes, []byte(doc.ID), []byte(doc.Created), []byte(doc.Updated), []byte(strconv.Itoa(doc.Version))}, []byte{})

	sort.SliceStable(doc.VerificationMethod, func(i, j int) bool {
		return doc.VerificationMethod[i].ID < doc.VerificationMethod[j].ID
	})
	for _, item := range doc.VerificationMethod {
		convertedBytes = bytes.Join([][]byte{convertedBytes, ConvertVMToBytes(item)}, []byte{})
	}

	convertedBytes = bytes.Join([][]byte{convertedBytes, []byte(doc.Authentication)}, []byte{})

	sort.SliceStable(doc.Service, func(i, j int) bool {
		return doc.Service[i].ID < doc.Service[j].ID
	})
	for _, item := range doc.Service {
		convertedBytes = bytes.Join([][]byte{convertedBytes, ConvertServiceToBytes(item)}, []byte{})
	}
	return convertedBytes
}

// convert VM to byte array. Used as part of converting document to bytes
func ConvertVMToBytes(vm VerificationMethod) []byte {
	var convertedBytes []byte

	convertedBytes = bytes.Join([][]byte{convertedBytes, []byte(vm.ID), []byte(vm.MethodType), []byte(vm.Controller), []byte(vm.BlockchainAccountId)}, []byte{})
	return convertedBytes
}

// convert service to byte array. Used as part of converting document to bytes
func ConvertServiceToBytes(service Service) []byte {
	var convertedBytes []byte

	convertedBytes = bytes.Join([][]byte{convertedBytes, []byte(service.ID), []byte(service.Type), []byte(service.ServiceEndpoint)}, []byte{})
	return convertedBytes
}
