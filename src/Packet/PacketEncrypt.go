package Packet

import (
	"config"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"time"

	logging "github.com/op/go-logging"
)

var (
	DEBUG          = true //Output debug info?
	err            error  //error interface
	Log            = logging.MustGetLogger("HoneyGO")
	publicKeyBytes []byte          //Key stored in byte array for packet delivery
	publicKey      *rsa.PublicKey  //PublicKey
	privateKey     *rsa.PrivateKey //PrivateKey
	//KeyLength      int             //Length of key array (should be 162)
	//ClientSharedSecret []byte            //Used for Authentication
	//ClientVerifyToken  []byte            //Used for Authentication
	//ServerVerifyToken  = make([]byte, 4) //Initialise a 4 element byte slice
)

const (
	ServerVerifyTokenLen = 4
)

//KeyGen - Generates KeyChain
func KeyGen() {
	SConfig = config.GetConfig()
	keys()
}

func keys() {
	var t time.Time
	if SConfig.Server.DEBUG {
		t = time.Now()
	}
	privateKey, err = rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		Log.Error(err.Error())
	}
	privateKey.Precompute()
	publicKey = &privateKey.PublicKey
	publicKeyBytes, err = x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		panic(err)
	}
	if SConfig.Server.DEBUG {
		Log.Info("Took Keys(): ", time.Since(t))
	}
	Log.Info("Key Generated!")
}

//--!!Not Used and will be removed later!!--//
//CreateEncryptionRequest - Creates the packet Encryption Request and sends to the client
/*func CreateEncryptionRequest(Connection *ClientConnection), CH chan bool) {
	Connection.KeepAlive()
	Log := logging.MustGetLogger("HoneyGO")
	Log.Debug("Login State, packetID 0x00")

	//Encryption Request
	//--Packet 0x01 S->C Start --//
	Log.Debug("Login State, packetID 0x01 Start")
	KeyLength = len(publicKeyBytes)
	//KeyLength Checks
	if KeyLength > 162 {
		Log.Warning("Key is bigger than expected!")
	}
	if KeyLength < 162 {
		Log.Warning("Key is smaller than expected!")
	} else {
		Log.Debug("Key Generated Successfully")
	}

	//PacketWrite - // NOTE: Later on the packet system will be redone in a more efficient manor where packets will be created in bulk
	writer := CreatePacketWriter(0x01)
	writer.WriteString("")                   //Empty;ServerID
	writer.WriteVarInt(int32(KeyLength))     //Key Byte array length
	writer.WriteArray(publicKeyBytes)        //Write Key byte Array
	writer.WriteVarInt(ServerVerifyTokenLen) //Always 4 on notchian servers
	rand.Read(ServerVerifyToken)             // Randomly Generate ServerVerifyToken
	writer.WriteArray(ServerVerifyToken)
	SendData(Connection, writer)
	Log.Debug("Encryption Request Sent")
}
*/
//SendData - Sends the data to the client
func SendData(Connection *ClientConnection, writer *PacketWriter) {
	Connection.Conn.Write(writer.GetPacket())
}

//To be able to retrieve the keychain because it runs within a goroutine
func GetPrivateKey() *rsa.PrivateKey {
	return privateKey
}

func GetPublicKey() *rsa.PublicKey {
	return publicKey
}

func GetPublicKeyBytes() []byte {
	return publicKeyBytes
}

func (Conn *ClientConnection) KeepAlive() {
	Conn.Conn.SetDeadline(time.Now().Add(time.Second * 10))
}
