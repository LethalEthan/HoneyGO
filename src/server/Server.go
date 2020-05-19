package server

import (
	"Packet"
	"VarTool"
	//	"VarTool"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"player"
	"strings"

	logging "github.com/op/go-logging"
)

//Define used variables
var (
	//Encryption Stuff
	publicKey           *rsa.PublicKey               //Public Key
	publicKeyBytes      []byte                       //PublicKey in a byte array for packet delivery and Auth check
	privateKey          *rsa.PrivateKey              //Like Do I need to comment this?
	Encryption          bool                         //TODO: Control via confighandler
	KeyLength           int                          //Keylength used by Encryption Request
	ClientSharedSecret  []byte                       //CSS
	ClientVerifyToken   []byte                       //CVT
	playername          string                       //For Authentication
	Log                 *logging.Logger              //Pretty obvious
	CurrentStatus       *ServerStatus                //ServerStatus Object
	ClientConnectionMap map[string]*ClientConnection //ClientConnectionMap - Map of connections, duh
	//Encryption stuff again
	DEBUG                 = true            //Output Debug info?
	GotDaKeys             = false           //Got dem keys?
	ClientSharedSecretLen = 128             //Initialise CSSL
	ClientVerifyTokenLen  = 128             //Initialise CVTL
	serverID              = ""              //this isn't used by mc anymore
	ServerVerifyToken     = make([]byte, 4) //Initialise a 4 element byte slice of cake
	//Chann                 = make(chan bool)
	PlayerMap = make(map[string]string) //Map Player to UUID
)

//REFERENCE: Play state goes from GameJoin.go -> SetDifficulty.go -> PlayerAbilities.go via goroutines
const (
	MinecraftVersion         = "1.15.2" //Supported MC version
	MinecraftProtocolVersion = 578      //Supported MC protocol Version
	ServerVerifyTokenLen     = 4        //Should always be 4 on notchian servers
)

func GetKeyChain() {
	privateKey = Packet.GetPrivateKey()
	publicKeyBytes = Packet.GetPublicKeyBytes()
	publicKey = Packet.GetPublicKey()
	GotDaKeys = true
}

func HandleConnection(Connection *ClientConnection) {
	if !GotDaKeys {
		GetKeyChain()
		Log.Debug("Got the keys!")
	}
	Log.Debug("Connection handler initiated")
	//Løøps
	for !Connection.isClosed {
		packet, packetSize, packetID, err := readPacketHeader(Connection)

		if err != nil {
			CloseClientConnection(Connection)
			Log.Error("Connection Terminated: " + err.Error())
			return
		}
		//DEBUG: print out all values
		if DEBUG {
			Log.Debug("Packet Size: ", packetSize)
			Log.Debug("Packet ID: ", packetID, "State: ", Connection.State)
			Log.Debugf("Packet Contains: %v\n", packet)
			Log.Debug("Direction: ", Connection.Direction)
			fmt.Print("")
		}
		//Create Packet Reader
		reader := Packet.CreatePacketReader(packet)
		//Packet Handling
		switch Connection.State {
		case HANDSHAKE: //Handle Handshake
			switch packetID {
			case 0x00:
				{
					//--Packet 0x00 S->C Start--//
					Hpacket, err := Packet.HandshakePacketCreate(packetSize, reader)
					if err != nil {
						CloseClientConnection(Connection) //You have been terminated
						Log.Error(err.Error())
					}
					Connection.KeepAlive() //Ah, ah. ah, ah stayin alive, stayin alive!
					Connection.State = int(Hpacket.NextState)
					break
					//--Packet 0x00 End--//
				}
			case 0xFE:
				{
					//--Packet 0xFE Legacy Ping Request --//
					Log.Warning("Legacy Ping Request received! - Terminated")
					CloseClientConnection(Connection)
					return
					//--Packet 0xFE End--//
				}
			}

		case STATUS: //Handle Status Request
			{
				switch packetID {
				case 0x00:
					{
						//--Packet 0x00 S->C Start--//
						Connection.KeepAlive()
						writer := Packet.CreatePacketWriter(0x00)
						marshaledStatus, err := json.Marshal(*CurrentStatus) //Sends status via json
						if err != nil {
							Log.Error(err.Error())
							CloseClientConnection(Connection)
							return
						}
						writer.WriteString(string(marshaledStatus))
						SendData(Connection, writer)
						break
					}
				case 0x01:
					{
						//--Packet 0x01 S->C Start--//
						Connection.KeepAlive()
						writer := Packet.CreatePacketWriter(0x01)
						Log.Debug("Status State, packetID 0x01")
						mirror, _ := reader.ReadLong()
						Log.Debug(mirror)
						writer.WriteLong(mirror)
						SendData(Connection, writer)
						CloseClientConnection(Connection)
						break
						//--Packet 0x01 End--//
					}
				}
			}
		case LOGIN: //Handle Login
			{
				switch packetID {
				case 0x00:
					{
						//--Packet 0x00 C->S Start--//
						Log.Debug("Login State, packetID 0x00")
						Connection.KeepAlive()
						playername, _ = reader.ReadString()
						PE := TranslatePacketStruct(Connection)
						go Packet.CreateEncryptionRequest(PE)
						//go CreatePacketSchedule(0, Connection)
						break
					}
				case 0x01:
					{
						//--Packet 0x01 C->S Start--//
						Connection.KeepAlive()
						//PP := TranslatePacketStruct(Connection)
						//go Packet.LoginPacketCreate(playername, publicKeyBytes, PP)
						Log.Debug("Login State, packetID 0x01")
						p := packet
						ClientSharedSecretLen = 128   //Should always be 128
						ClientSharedSecret = p[2:130] //Find the 128 bytes in the whole byte array
						ClientVerifyToken = p[132:]   //Find the 128 bytes in whole byte array
						Connection.KeepAlive()
						//Decrypt Shared Secret
						decryptSS, err := rsa.DecryptPKCS1v15(rand.Reader, privateKey, ClientSharedSecret)
						if err != nil {
							fmt.Print(err)
						}
						ClientSharedSecret = decryptSS //Set the decrypted value
						ClientSharedSecretLen = len(ClientSharedSecret)
						//Basic check to see whether it's 16 bytes
						if ClientSharedSecretLen != 16 {
							Log.Warning("Shared Secret Length is NOT 16 bytes :(")
						} else {
							Log.Info("ClientSharedSecret Recieved Successfully")
						}
						//Decrypt Verify Token
						decryptVT, err := rsa.DecryptPKCS1v15(rand.Reader, privateKey, ClientVerifyToken)
						if err != nil {
							fmt.Print(err)
						}
						Connection.KeepAlive()
						ClientVerifyTokenLen = len(decryptVT)
						if ServerVerifyTokenLen != ClientVerifyTokenLen {
							Log.Warning("Encryption Failed!")
						} else {
							Log.Info("Encryption Successful!")
						}
						//Authenticate Player
						var Auth string
						//Check if playermap has any data
						//if PlayerMap != nil {
						if val, tmp := PlayerMap[playername]; tmp { //checks if map has the value
							Auth = val //Set auth to value
						} else { //If uuid isn't found, get it
							//2 attempts to get UUID
							Auth, err = Authenticate(playername, serverID, ClientSharedSecret, publicKeyBytes)
							if err != nil {
								Log.Error("Authentication Failed, trying second time")
								Auth, err = Authenticate(playername, serverID, ClientSharedSecret, publicKeyBytes)
								if err != nil {
									Log.Error("Authentication failed on second attempt, closing connection")
									CloseClientConnection(Connection)
								} else { //If no errors cache uuid in map
									PlayerMap[playername] = Auth
								}
							} else { //If no erros cache uuid in map
								PlayerMap[playername] = Auth
							}
						}
						Log.Debug(playername, "[", Auth, "]")
						//--Packer 0x01 End--//

						//--Packet 0x02 S->C Start--//
						writer := Packet.CreatePacketWriter(0x02)
						Log.Debug("Playername: ", playername)
						writer.WriteString(Auth)
						writer.WriteString(playername)
						//UUID Cache
						//PlayerMap[playername] = Auth
						//DEBUG: REMOVE ME
						Log.Debug("PlayerMap: ", PlayerMap)
						Log.Debug("PlayerData:", PlayerMap[playername])
						SendData(Connection, writer)
						Connection.State = PLAY
						PC := TranslatePlayerStruct(Connection) //Translates Server.ClientConnection -> player.ClientConnection
						//NOTE: goroutines are light weight threads that can be reused with the same stack created before,
						//this will be useful when multiple clients connect but with some added memory usage

						/*TODO: Use channels to signal when a goroutine has finished and tell the other goroutines
						to send the data to the client, this means that all the packets will be created and ready upon recieving the "all clear"
						Which would theortically reduce latency and the time needed to craft the packets*/
						//Although there maybe a possibility of a lockup if something goes wrong and the data isn't recieved via the channel
						C := make(chan bool)
						go player.CreateGameJoin(PC, C) //Creates JoinGame packet AND SetDifficulty AND Player Abilities via go routines
						Log.Debug("END")
						//SendData(Connection, writer26)
						break
					}
				case 0x02:
					{
						Log.Debug("Login State, packet 0x02")
						break
					}
				case 0x05:
					{
						Log.Debug("Packet 0x05")
						break
					}
				}
			}
		case PLAY:
			{
				switch packetID {
				case 0x00:
					{
						Log.Debug("Play State, packet 0x00")
						break
					}
				case 0x01:
					{
						Log.Debug("Play State, Packet 0x01")
						break
					}
				case 0x02:
					{
						Log.Debug("Play State, Packet 0x02")
						break
					}
				case 0x03:
					{
						Log.Debug("Play State, Packet 0x03")
						break
					}
				case 0x04:
					{
						Log.Debug("Play State, Packet 0x04")
						break
					}
				case 0x05:
					{
						Log.Debug("Play State, Packet 0x05")
						break
					}
				default:
					for {
						Log.Fatal("Play Packet recieved")
					}
				}
			}
		default:
			Log.Debug("oo")
		}
	}
}

//SendData - Sends the data to the client
func SendData(Connection *ClientConnection, writer *Packet.PacketWriter) {
	Connection.Conn.Write(writer.GetPacket())
}

//getPacketData - Reads incoming packets and returns data in a byte array to which ever function requires it
func getPacketData(Conn net.Conn) ([]byte, error) {
	return ioutil.ReadAll(Conn)
}

//Server ClientConn -> Player ClientConn
func TranslatePlayerStruct(Conn *ClientConnection) *player.ClientConnection {
	PC := new(player.ClientConnection)
	PC.Conn = Conn.Conn
	PC.State = Conn.State
	return PC
}

//Server ClientConn -> Packet ClientConn
func TranslatePacketStruct(Conn *ClientConnection) *Packet.ClientConnection {
	PE := new(Packet.ClientConnection)
	PE.Conn = Conn.Conn
	PE.State = Conn.State
	return PE
}

//readPacketHeader - Reads the packet Header for Packet ID and size info
func readPacketHeader(Conn *ClientConnection) ([]byte, int32, int32, error) {
	//Read Packet size
	packetSize, err := VarTool.ParseVarIntFromConnection(Conn.Conn)
	if err != nil {
		return nil, 0, 0, err //Return nothing on error
	}
	//Information used from wiki.vg
	//Handling Handshake
	if packetSize == 254 && Conn.State == HANDSHAKE {
		PreBufferSize := 30 //Create PreBuffer
		PreBuffer := make([]byte, PreBufferSize)
		Conn.Conn.Read(PreBuffer)
		//PostBuffer -> Big endian
		PostBufferSize := int(binary.BigEndian.Uint16(PreBuffer[25:]))
		PostBuffer := make([]byte, PostBufferSize)
		size := PreBufferSize + PostBufferSize
		Conn.Conn.Read(PostBuffer)
		return append(PreBuffer, PostBuffer...), int32(size), 0xFE, nil
	}

	packetID, err := VarTool.ParseVarIntFromConnection(Conn.Conn)

	if err != nil {
		return nil, 0, 0, err
	}

	if packetSize-1 == 0 {
		return nil, packetSize, packetID, nil
	}
	packet := make([]byte, packetSize-1)
	Conn.Conn.Read(packet)
	return packet, packetSize - 1, packetID, nil
}

var ErrorAuthFailed = errors.New("Authentication failed")

type jsonResponse struct {
	ID string `json:"id"`
}

func Authenticate(username string, serverID string, sharedSecret, publicKey []byte) (string, error) {
	//A hash is created using the shared secret and public key and is sent to the mojang sessionserver
	//The server returns the data about the player including the player's skin blob
	//Again I cannot thank enough wiki.vg, this is based off one of the linked java gists by Drew DeVault; thank you for the gist that I used to base this off
	sha := sha1.New()
	sha.Write([]byte(serverID))
	sha.Write(sharedSecret)
	sha.Write(publicKey)
	hash := sha.Sum(nil)

	negative := (hash[0] & 0x80) == 0x80
	if negative {
		twosCompliment(hash)
	}

	buf := hex.EncodeToString(hash)
	if negative {
		buf = "-" + buf
	}
	hashString := strings.TrimLeft(buf, "0")

	response, err := http.Get(fmt.Sprintf("https://sessionserver.mojang.com/session/minecraft/hasJoined?username=%s&serverId=%s", username, hashString))
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	dec := json.NewDecoder(response.Body)
	res := &jsonResponse{}
	err = dec.Decode(res)
	if err != nil {
		return "", ErrorAuthFailed
	}

	if len(res.ID) != 32 {
		return "", ErrorAuthFailed
	}
	hyphenater := res.ID[0:8] + "-" + res.ID[8:12] + "-" + res.ID[12:16] + "-" + res.ID[16:20] + "-" + res.ID[20:]
	res.ID = hyphenater
	return res.ID, nil
}

func twosCompliment(p []byte) {
	carry := true
	for i := len(p) - 1; i >= 0; i-- {
		p[i] = ^p[i]
		if carry {
			carry = p[i] == 0xFF
			p[i]++
		}
	}
}
