package server

import (
	"Packet"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"player"
	"time"
)

var KC = true

func Handle_MC1_15_2(Connection *ClientConnection, PH PacketHeader) {
	if KC {
		GetKeyChain()
		KC = false
	}
	Log.Info("Connection handler for MC 1.15.2 initiated")
	CurrentStatus = CreateStatusObject(578, "1.15.2")
	//Log.Warning(publicKey, privateKey)
	if publicKey == nil || privateKey == nil {
		panic("Keys have been thanos snapped")
	}

	for !Connection.isClosed {
		var err error
		PH.packet, PH.packetSize, PH.packetID, err = readPacketHeader(Connection)
		if err != nil {
			CloseClientConnection(Connection)
			Log.Error("Connection Terminated: " + err.Error())
			return
		}
		//DEBUG: output debug info
		if DEBUG {
			Log.Debug("Packet Size: ", PH.packetSize)
			Log.Debug("Packet ID: ", PH.packetID, "State: ", Connection.State)
			Log.Debugf("Packet Contains: %v\n", PH.packet)
			Log.Debug("Protocol: ", PH.protocol)
			Log.Debug("Direction: ", Connection.Direction) //TBD
			fmt.Print("")
		}
		//Create Packet Reader
		reader := Packet.CreatePacketReader(PH.packet)
		//Packet Handling
		switch Connection.State {
		case STATUS: //Handle Status Request
			{
				switch PH.packetID {
				case 0x00:
					{
						//--Packet 0x00 S->C Start--// Request Status
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
					}
				case 0x01:
					{
						//--Packet 0x01 S->C Start--// Ping
						Connection.KeepAlive()
						writer := Packet.CreatePacketWriter(0x01)
						Log.Debug("Status State, packetID 0x01")
						mirror, _ := reader.ReadLong()
						Log.Debug("Mirror:", mirror)
						writer.WriteLong(mirror)
						SendData(Connection, writer)
						CloseClientConnection(Connection)
						//--Packet 0x01 End--//
					}
				}
			}
		case LOGIN: //Handle Login
			{
				switch PH.packetID {
				case 0x00:
					{
						//--Packet 0x00 C->S Start--// Login Start (Player Username)
						Log.Debug("Login State, packetID 0x00")
						Connection.KeepAlive()
						playername, _ = reader.ReadString()
						//--Packet 0x01 S->C --// Encryption Request
						CreateEncryptionRequest(Connection)
					}
				case 0x01:
					{
						//--Packet 0x01 S->C Start--//
						//EncryptionResponse
						Connection.KeepAlive()
						Log.Debug("Login State, packetID 0x01")
						Log.Debug("PacketSIZE: ", PH.packetSize)
						if PH.packetSize > 260 {
							CloseClientConnection(Connection)
							return
						}
						ClientSharedSecretLen = 128           //Should always be 128 when encrypted
						ClientSharedSecret = PH.packet[2:130] //Find the first 128 bytes in the whole byte array
						ClientVerifyToken = PH.packet[132:]   //Find the second 128 bytes in whole byte array
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
						var Auth string
						//--Authentication Stuff--//
						//Authenticate Player
						//Check if playermap has any data -- UUID Caching
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
						//DEBUG: REMOVE ME
						//Log.Debug("PlayerMap: ", PlayerMap)
						//Log.Debug("PlayerData:", PlayerMap[playername])
						time.Sleep(5000000) //DEBUG:Add delay -- remove me later
						SendData(Connection, writer)

						///Entity ID Handling///
						PlayerConnMap[Connection.Conn] = playername //link connection to player
						player.InitPlayer(playername, Auth /*, player.PlayerEntityMap[playername]*/, 1)
						player.GetPlayerByID(player.PlayerEntityMap[playername])
						EID := player.PlayerEntityMap[playername]
						ConnPlayerMap[EID] = Connection.Conn
						//go player.GCPlayer() //DEBUG: REMOVE ME LATER
						//--//
						Connection.State = PLAY
						//worldtime.
						//C := make(chan bool)
						PC := &player.ClientConnection{Connection.Conn, Connection.State, Connection.isClosed}
						player.CreateGameJoin(PC, player.PlayerEntityMap[playername])
						player.CreateSetDiff(PC)
						player.CreatePlayerAbilities(PC)
						Log.Debug("END")
						CloseClientConnection(Connection)
						Disconnect(playername)
						//time.Sleep(60000000)
						//CloseClientConnection(Connection)
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
			//Play will be handled by another package/function
		case PLAY:
			{
				switch PH.packetID {
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