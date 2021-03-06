package player

import (
	"Packet"
	"net"
	"time"
)

const (
	Nether    = -1
	Overworld = 0
	End       = 1
)

var Dimension = Overworld

type ClientConnection struct {
	Conn   net.Conn
	State  int
	Closed bool
}

//GameJoin - Structure of the JoinGame packet
type GameJoin struct {
	EntityID            uint32 //Players EntityID
	GameMode            uint8  //0: Survival, 1: Creative, 2: Adventure, 3: Spectator. Bit 3 (0x8) is the hardcore flag.
	Dimension           int    //See connstants above
	HashedSeed          int64  //First 8 bytes of the SHA-256 hash of world seed
	MaxPlayers          uint8  //Used to be used but according to wiki.vg it's no longer used
	LevelType           string //Max 16 length: default, flat, largeBiomes etc
	ViewDistance        VarInt //RenderDistance (2-32)
	ReducedDebugInfo    bool
	EnableRespawnScreen bool //Set false when doImmediateRespawn gamerule is true
}

func CreateGameJoin(Conn *ClientConnection /*, C chan bool*/, EID uint32) {
	//log.Debug("Packet Play, 0x26 Created")
	//Conn.KeepAlive() //KeepAlive
	GJ := &GameJoin{EID, Creative, 0, 53873645, 20, "default", 16, false, true}
	//log.Debug("GJ:", GJ)
	//No easy way to do this without this mess, a packet system re-write will be done in the future
	writer := Packet.CreatePacketWriter(0x26)
	writer.WriteInt(int32(GJ.EntityID)) //In PW it goes from int32 -> uint32 this will be different when packethandler rewrite is complete
	writer.WriteUnsignedByte(GJ.GameMode)
	writer.WriteInt(int32(GJ.Dimension))
	writer.WriteLong(GJ.HashedSeed)
	writer.WriteUnsignedByte(0)
	writer.WriteString("default")
	writer.WriteVarInt(16)
	writer.WriteBoolean(false)
	writer.WriteBoolean(true)
	SendData(Conn, writer)
	//log.Debug("GameJoin Packet sent")
	//log.Debug("GOR:", runtime.NumGoroutine())
	//CreateSetDiff(Conn) //Creates SetDifficultyPacket
	//C <- true
}

// func CreateGameJoinInstance(EntityID uint32) {
// 	Player := GetPlayerByID(EntityID)
// 	GJ := &GameJoin{2, Player.GameMode, 0, 12345, 20, "default", 16, false, true}
// 	//log.Debug("GJ:", GJ)
// }

func SendData(Connection *ClientConnection, writer *Packet.PacketWriter) {
	Connection.Conn.Write(writer.GetPacket())
}

func (Conn *ClientConnection) KeepAlive() {
	Conn.Conn.SetDeadline(time.Now().Add(time.Second * 10))
}

// func fetchtype(t *GameJoin) {
// 	fmt.Print(reflect.TypeOf(t.EntityID))
//
// }
