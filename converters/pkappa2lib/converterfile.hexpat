#include <std/io.pat>
#include <std/mem.pat>

struct VarInt {
	u8 array[while($ == addressof(this) || std::mem::read_unsigned($-1, 1) & 0x80 != 0)] [[hidden]];
} [[sealed, format("impl::format_varint"), transform("impl::transform_varint")]];

namespace impl {

    fn transform_varint_array(auto array) {
        u128 res = array[0] & 0x7f;
        for(u8 i = 1, array[i-1] & 0x80 != 0, i+=1) {
            res <<= 7;
            res |= u64(array[i] & 0x7f);
        }
        return res;
    };

    fn transform_varint(auto varint) {
        return impl::transform_varint_array(varint.array);
    };

    fn format_varint(auto varint) {
        u128 res = impl::transform_varint_array(varint.array);
        return std::format("{} ({:#x})", res, res);
    };
}

fn CountBytes(u8 type, VarInt sizes) {
    u64 res = 0;
    u8 direction = 0;
    bool prevNull = false;
    for(u64 i = 0, sizes[i] != 0 && !prevNull, i+=1) {
        if (sizes[i] == 0) {
            prevNull = true;
            continue;
        }
        prevNull = false;
        if (direction == type) {
            res += sizes[i];
        }
        direction ^= 1;
    }
    return res;
};

struct Stream {
    u64 StreamID;
    VarInt sizes[while(addressof(PacketCount)+8 == $ || std::mem::read_unsigned($-1, 1) != 0 || std::mem::read_unsigned($-2, 1) != 0)];
    u8 ClientData[CountBytes(0, sizes)];
    u8 ServerData[CountBytes(1, sizes)];
};

// Stream stream[while(!std::mem::eof())] @ 0x00;
Stream streams[1] @ 0x0;
