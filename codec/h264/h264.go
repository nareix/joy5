package h264

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"math"
	"sort"

	"github.com/nareix/joy5/utils/bits"
	"github.com/nareix/joy5/utils/bits/pio"
)

const (
	NALU_NONIDR = 1
	NALU_IDR    = 5
	NALU_SEI    = 6
	NALU_SPS    = 7
	NALU_PPS    = 8
	NALU_AUD    = 9

	SEI_TYPE_USER_DATA_UNREGISTERED = 5
)

const (
	FF_PROFILE_H264_CONSTRAINED = (1 << 12) // 8+4; constraint_set1_flag
	FF_PROFILE_H264_INTRA       = (1 << 10) // 8+2; constraint_set3_flag

	FF_PROFILE_H264_BASELINE             = 66
	FF_PROFILE_H264_CONSTRAINED_BASELINE = (66 | FF_PROFILE_H264_CONSTRAINED)
	FF_PROFILE_H264_MAIN                 = 77
	FF_PROFILE_H264_EXTENDED             = 88
	FF_PROFILE_H264_HIGH                 = 100
	FF_PROFILE_H264_HIGH_10              = 110
	FF_PROFILE_H264_HIGH_10_INTRA        = (110 | FF_PROFILE_H264_INTRA)
	FF_PROFILE_H264_MULTIVIEW_HIGH       = 118
	FF_PROFILE_H264_HIGH_422             = 122
	FF_PROFILE_H264_HIGH_422_INTRA       = (122 | FF_PROFILE_H264_INTRA)
	FF_PROFILE_H264_STEREO_HIGH          = 128
	FF_PROFILE_H264_HIGH_444             = 144
	FF_PROFILE_H264_HIGH_444_PREDICTIVE  = 244
	FF_PROFILE_H264_HIGH_444_INTRA       = (244 | FF_PROFILE_H264_INTRA)
	FF_PROFILE_H264_CAVLC_444            = 44
)

var ProfileMap = map[uint]string{
	FF_PROFILE_H264_BASELINE:             "Baseline",
	FF_PROFILE_H264_CONSTRAINED_BASELINE: "Constrained Baseline",
	FF_PROFILE_H264_MAIN:                 "Main",
	FF_PROFILE_H264_EXTENDED:             "Extended",
	FF_PROFILE_H264_HIGH:                 "High",
	FF_PROFILE_H264_HIGH_10:              "High 10",
	FF_PROFILE_H264_HIGH_10_INTRA:        "High 10 Intra",
	FF_PROFILE_H264_HIGH_422:             "High 4:2:2",
	FF_PROFILE_H264_HIGH_422_INTRA:       "High 4:2:2 Intra",
	FF_PROFILE_H264_HIGH_444:             "High 4:4:4",
	FF_PROFILE_H264_HIGH_444_PREDICTIVE:  "High 4:4:4 Predictive",
	FF_PROFILE_H264_HIGH_444_INTRA:       "High 4:4:4 Intra",
	FF_PROFILE_H264_CAVLC_444:            "CAVLC 4:4:4",
	FF_PROFILE_H264_MULTIVIEW_HIGH:       "Multiview High",
	FF_PROFILE_H264_STEREO_HIGH:          "Stereo High",
}

func NALUType(b []byte) byte {
	if len(b) > 0 {
		return b[0] & 0x1f
	}
	return 0
}

func NALUTypeString(i byte) string {
	switch i {
	case NALU_NONIDR:
		return "NONIDR"
	case NALU_IDR:
		return "IDR"
	case NALU_SEI:
		return "SEI"
	case NALU_SPS:
		return "SPS"
	case NALU_PPS:
		return "PPS"
	case NALU_AUD:
		return "AUD"
	default:
		return fmt.Sprint(i)
	}
}

func IsDataNALU(b []byte) bool {
	typ := NALUType(b)
	return typ >= 1 && typ <= 5
}

/*
From: http://stackoverflow.com/questions/24884827/possible-locations-for-sequence-picture-parameter-sets-for-h-264-stream

First off, it's important to understand that there is no single standard H.264 elementary bitstream format. The specification document does contain an Annex, specifically Annex B, that describes one possible format, but it is not an actual requirement. The standard specifies how video is encoded into individual packets. How these packets are stored and transmitted is left open to the integrator.

1. Annex B
Network Abstraction Layer Units
The packets are called Network Abstraction Layer Units. Often abbreviated NALU (or sometimes just NAL) each packet can be individually parsed and processed. The first byte of each NALU contains the NALU type, specifically bits 3 through 7. (bit 0 is always off, and bits 1-2 indicate whether a NALU is referenced by another NALU).

There are 19 different NALU types defined separated into two categories, VCL and non-VCL:

VCL, or Video Coding Layer packets contain the actual visual information.
Non-VCLs contain metadata that may or may not be required to decode the video.
A single NALU, or even a VCL NALU is NOT the same thing as a frame. A frame can be ‘sliced’ into several NALUs. Just like you can slice a pizza. One or more slices are then virtually grouped into a Access Units (AU) that contain one frame. Slicing does come at a slight quality cost, so it is not often used.

Below is a table of all defined NALUs.

0      Unspecified                                                    non-VCL
1      Coded slice of a non-IDR picture                               VCL
2      Coded slice data partition A                                   VCL
3      Coded slice data partition B                                   VCL
4      Coded slice data partition C                                   VCL
5      Coded slice of an IDR picture                                  VCL
6      Supplemental enhancement information (SEI)                     non-VCL
7      Sequence parameter set                                         non-VCL
8      Picture parameter set                                          non-VCL
9      Access unit delimiter                                          non-VCL
10     End of sequence                                                non-VCL
11     End of stream                                                  non-VCL
12     Filler data                                                    non-VCL
13     Sequence parameter set extension                               non-VCL
14     Prefix NAL unit                                                non-VCL
15     Subset sequence parameter set                                  non-VCL
16     Depth parameter set                                            non-VCL
17..18 Reserved                                                       non-VCL
19     Coded slice of an auxiliary coded picture without partitioning non-VCL
20     Coded slice extension                                          non-VCL
21     Coded slice extension for depth view components                non-VCL
22..23 Reserved                                                       non-VCL
24..31 Unspecified                                                    non-VCL
There are a couple of NALU types where having knowledge of may be helpful later.

Sequence Parameter Set (SPS). This non-VCL NALU contains information required to configure the decoder such as profile, level, resolution, frame rate.
Picture Parameter Set (PPS). Similar to the SPS, this non-VCL contains information on entropy coding mode, slice groups, motion prediction and deblocking filters.
Instantaneous Decoder Refresh (IDR). This VCL NALU is a self contained image slice. That is, an IDR can be decoded and displayed without referencing any other NALU save SPS and PPS.
Access Unit Delimiter (AUD). An AUD is an optional NALU that can be use to delimit frames in an elementary stream. It is not required (unless otherwise stated by the container/protocol, like TS), and is often not included in order to save space, but it can be useful to finds the start of a frame without having to fully parse each NALU.
NALU Start Codes
A NALU does not contain is its size. Therefore simply concatenating the NALUs to create a stream will not work because you will not know where one stops and the next begins.

The Annex B specification solves this by requiring ‘Start Codes’ to precede each NALU. A start code is 2 or 3 0x00 bytes followed with a 0x01 byte. e.g. 0x000001 or 0x00000001.

The 4 byte variation is useful for transmission over a serial connection as it is trivial to byte align the stream by looking for 31 zero bits followed by a one. If the next bit is 0 (because every NALU starts with a 0 bit), it is the start of a NALU. The 4 byte variation is usually only used for signaling random access points in the stream such as a SPS PPS AUD and IDR Where as the 3 byte variation is used everywhere else to save space.

Emulation Prevention Bytes
Start codes work because the four byte sequences 0x000000, 0x000001, 0x000002 and 0x000003 are illegal within a non-RBSP NALU. So when creating a NALU, care is taken to escape these values that could otherwise be confused with a start code. This is accomplished by inserting an ‘Emulation Prevention’ byte 0x03, so that 0x000001 becomes 0x00000301.

When decoding, it is important to look for and ignore emulation prevention bytes. Because emulation prevention bytes can occur almost anywhere within a NALU, it is often more convenient in documentation to assume they have already been removed. A representation without emulation prevention bytes is called Raw Byte Sequence Payload (RBSP).

Example
Let's look at a complete example.

0x0000 | 00 00 00 01 67 64 00 0A AC 72 84 44 26 84 00 00
0x0010 | 03 00 04 00 00 03 00 CA 3C 48 96 11 80 00 00 00
0x0020 | 01 68 E8 43 8F 13 21 30 00 00 01 65 88 81 00 05
0x0030 | 4E 7F 87 DF 61 A5 8B 95 EE A4 E9 38 B7 6A 30 6A
0x0040 | 71 B9 55 60 0B 76 2E B5 0E E4 80 59 27 B8 67 A9
0x0050 | 63 37 5E 82 20 55 FB E4 6A E9 37 35 72 E2 22 91
0x0060 | 9E 4D FF 60 86 CE 7E 42 B7 95 CE 2A E1 26 BE 87
0x0070 | 73 84 26 BA 16 36 F4 E6 9F 17 DA D8 64 75 54 B1
0x0080 | F3 45 0C 0B 3C 74 B3 9D BC EB 53 73 87 C3 0E 62
0x0090 | 47 48 62 CA 59 EB 86 3F 3A FA 86 B5 BF A8 6D 06
0x00A0 | 16 50 82 C4 CE 62 9E 4E E6 4C C7 30 3E DE A1 0B
0x00B0 | D8 83 0B B6 B8 28 BC A9 EB 77 43 FC 7A 17 94 85
0x00C0 | 21 CA 37 6B 30 95 B5 46 77 30 60 B7 12 D6 8C C5
0x00D0 | 54 85 29 D8 69 A9 6F 12 4E 71 DF E3 E2 B1 6B 6B
0x00E0 | BF 9F FB 2E 57 30 A9 69 76 C4 46 A2 DF FA 91 D9
0x00F0 | 50 74 55 1D 49 04 5A 1C D6 86 68 7C B6 61 48 6C
0x0100 | 96 E6 12 4C 27 AD BA C7 51 99 8E D0 F0 ED 8E F6
0x0110 | 65 79 79 A6 12 A1 95 DB C8 AE E3 B6 35 E6 8D BC
0x0120 | 48 A3 7F AF 4A 28 8A 53 E2 7E 68 08 9F 67 77 98
0x0130 | 52 DB 50 84 D6 5E 25 E1 4A 99 58 34 C7 11 D6 43
0x0140 | FF C4 FD 9A 44 16 D1 B2 FB 02 DB A1 89 69 34 C2
0x0150 | 32 55 98 F9 9B B2 31 3F 49 59 0C 06 8C DB A5 B2
0x0160 | 9D 7E 12 2F D0 87 94 44 E4 0A 76 EF 99 2D 91 18
0x0170 | 39 50 3B 29 3B F5 2C 97 73 48 91 83 B0 A6 F3 4B
0x0180 | 70 2F 1C 8F 3B 78 23 C6 AA 86 46 43 1D D7 2A 23
0x0190 | 5E 2C D9 48 0A F5 F5 2C D1 FB 3F F0 4B 78 37 E9
0x01A0 | 45 DD 72 CF 80 35 C3 95 07 F3 D9 06 E5 4A 58 76
0x01B0 | 03 6C 81 20 62 45 65 44 73 BC FE C1 9F 31 E5 DB
0x01C0 | 89 5C 6B 79 D8 68 90 D7 26 A8 A1 88 86 81 DC 9A
0x01D0 | 4F 40 A5 23 C7 DE BE 6F 76 AB 79 16 51 21 67 83
0x01E0 | 2E F3 D6 27 1A 42 C2 94 D1 5D 6C DB 4A 7A E2 CB
0x01F0 | 0B B0 68 0B BE 19 59 00 50 FC C0 BD 9D F5 F5 F8
0x0200 | A8 17 19 D6 B3 E9 74 BA 50 E5 2C 45 7B F9 93 EA
0x0210 | 5A F9 A9 30 B1 6F 5B 36 24 1E 8D 55 57 F4 CC 67
0x0220 | B2 65 6A A9 36 26 D0 06 B8 E2 E3 73 8B D1 C0 1C
0x0230 | 52 15 CA B5 AC 60 3E 36 42 F1 2C BD 99 77 AB A8
0x0240 | A9 A4 8E 9C 8B 84 DE 73 F0 91 29 97 AE DB AF D6
0x0250 | F8 5E 9B 86 B3 B3 03 B3 AC 75 6F A6 11 69 2F 3D
0x0260 | 3A CE FA 53 86 60 95 6C BB C5 4E F3

This is a complete AU containing 3 NALUs. As you can see, we begin with a Start code followed by an SPS (SPS starts with 67). Within the SPS, you will see two Emulation Prevention bytes. Without these bytes the illegal sequence 0x000000 would occur at these positions. Next you will see a start code followed by a PPS (PPS starts with 68) and one final start code followed by an IDR slice. This is a complete H.264 stream. If you type these values into a hex editor and save the file with a .264 extension, you will be able to convert it to this image:

Lena

Annex B is commonly used in live and streaming formats such as transport streams, over the air broadcasts, and DVDs. In these formats it is common to repeat the SPS and PPS periodically, usually preceding every IDR thus creating a random access point for the decoder. This enables the ability to join a stream already in progress.

2. AVCC
The other common method of storing an H.264 stream is the AVCC format. In this format, each NALU is preceded with its length (in big endian format). This method is easier to parse, but you lose the byte alignment features of Annex B. Just to complicate things, the length may be encoded using 1, 2 or 4 bytes. This value is stored in a header object. This header is often called ‘extradata’ or ‘sequence header’. Its basic format is as follows:

bits
8   version ( always 0x01 )
8   avc profile ( sps[0][1] )
8   avc compatibility ( sps[0][2] )
8   avc level ( sps[0][3] )
6   reserved ( all bits on )
2   NALULengthSizeMinusOne
3   reserved ( all bits on )
5   number of SPS NALUs (usually 1)
repeated once per SPS:
  16         SPS size
	variable   SPS NALU data
8   number of PPS NALUs (usually 1)
repeated once per PPS
  16         PPS size
  variable   PPS NALU data

Using the same example above, the AVCC extradata will look like this:

0x0000 | 01 64 00 0A FF E1 00 19 67 64 00 0A AC 72 84 44
0x0010 | 26 84 00 00 03 00 04 00 00 03 00 CA 3C 48 96 11
0x0020 | 80 01 00 07 68 E8 43 8F 13 21 30

You will notice SPS and PPS is now stored out of band. That is, separate from the elementary stream data. Storage and transmission of this data is the job of the file container, and beyond the scope of this document. Notice that even though we are not using start codes, emulation prevention bytes are still inserted.

Additionally, there is a new variable called NALULengthSizeMinusOne. This confusingly named variable tells us how many bytes to use to store the length of each NALU. So, if NALULengthSizeMinusOne is set to 0, then each NALU is preceded with a single byte indicating its length. Using a single byte to store the size, the max size of a NALU is 255 bytes. That is obviously pretty small. Way too small for an entire key frame. Using 2 bytes gives us 64k per NALU. It would work in our example, but is still a pretty low limit. 3 bytes would be perfect, but for some reason is not universally supported. Therefore, 4 bytes is by far the most common, and it is what we used here:

0x0000 | 00 00 02 41 65 88 81 00 05 4E 7F 87 DF 61 A5 8B
0x0010 | 95 EE A4 E9 38 B7 6A 30 6A 71 B9 55 60 0B 76 2E
0x0020 | B5 0E E4 80 59 27 B8 67 A9 63 37 5E 82 20 55 FB
0x0030 | E4 6A E9 37 35 72 E2 22 91 9E 4D FF 60 86 CE 7E
0x0040 | 42 B7 95 CE 2A E1 26 BE 87 73 84 26 BA 16 36 F4
0x0050 | E6 9F 17 DA D8 64 75 54 B1 F3 45 0C 0B 3C 74 B3
0x0060 | 9D BC EB 53 73 87 C3 0E 62 47 48 62 CA 59 EB 86
0x0070 | 3F 3A FA 86 B5 BF A8 6D 06 16 50 82 C4 CE 62 9E
0x0080 | 4E E6 4C C7 30 3E DE A1 0B D8 83 0B B6 B8 28 BC
0x0090 | A9 EB 77 43 FC 7A 17 94 85 21 CA 37 6B 30 95 B5
0x00A0 | 46 77 30 60 B7 12 D6 8C C5 54 85 29 D8 69 A9 6F
0x00B0 | 12 4E 71 DF E3 E2 B1 6B 6B BF 9F FB 2E 57 30 A9
0x00C0 | 69 76 C4 46 A2 DF FA 91 D9 50 74 55 1D 49 04 5A
0x00D0 | 1C D6 86 68 7C B6 61 48 6C 96 E6 12 4C 27 AD BA
0x00E0 | C7 51 99 8E D0 F0 ED 8E F6 65 79 79 A6 12 A1 95
0x00F0 | DB C8 AE E3 B6 35 E6 8D BC 48 A3 7F AF 4A 28 8A
0x0100 | 53 E2 7E 68 08 9F 67 77 98 52 DB 50 84 D6 5E 25
0x0110 | E1 4A 99 58 34 C7 11 D6 43 FF C4 FD 9A 44 16 D1
0x0120 | B2 FB 02 DB A1 89 69 34 C2 32 55 98 F9 9B B2 31
0x0130 | 3F 49 59 0C 06 8C DB A5 B2 9D 7E 12 2F D0 87 94
0x0140 | 44 E4 0A 76 EF 99 2D 91 18 39 50 3B 29 3B F5 2C
0x0150 | 97 73 48 91 83 B0 A6 F3 4B 70 2F 1C 8F 3B 78 23
0x0160 | C6 AA 86 46 43 1D D7 2A 23 5E 2C D9 48 0A F5 F5
0x0170 | 2C D1 FB 3F F0 4B 78 37 E9 45 DD 72 CF 80 35 C3
0x0180 | 95 07 F3 D9 06 E5 4A 58 76 03 6C 81 20 62 45 65
0x0190 | 44 73 BC FE C1 9F 31 E5 DB 89 5C 6B 79 D8 68 90
0x01A0 | D7 26 A8 A1 88 86 81 DC 9A 4F 40 A5 23 C7 DE BE
0x01B0 | 6F 76 AB 79 16 51 21 67 83 2E F3 D6 27 1A 42 C2
0x01C0 | 94 D1 5D 6C DB 4A 7A E2 CB 0B B0 68 0B BE 19 59
0x01D0 | 00 50 FC C0 BD 9D F5 F5 F8 A8 17 19 D6 B3 E9 74
0x01E0 | BA 50 E5 2C 45 7B F9 93 EA 5A F9 A9 30 B1 6F 5B
0x01F0 | 36 24 1E 8D 55 57 F4 CC 67 B2 65 6A A9 36 26 D0
0x0200 | 06 B8 E2 E3 73 8B D1 C0 1C 52 15 CA B5 AC 60 3E
0x0210 | 36 42 F1 2C BD 99 77 AB A8 A9 A4 8E 9C 8B 84 DE
0x0220 | 73 F0 91 29 97 AE DB AF D6 F8 5E 9B 86 B3 B3 03
0x0230 | B3 AC 75 6F A6 11 69 2F 3D 3A CE FA 53 86 60 95
0x0240 | 6C BB C5 4E F3

An advantage to this format is the ability to configure the decoder at the start and jump into the middle of a stream. This is a common use case where the media is available on a random access medium such as a hard drive, and is therefore used in common container formats such as MP4 and MKV.
*/

var StartCodeBytes = []byte{0, 0, 1}
var AUDBytes = []byte{0, 0, 0, 1, 0x9, 0xf0, 0, 0, 0, 1} // AUD

func CheckNALUsType(b []byte) (typ int) {
	_, typ = SplitNALUs(b)
	return
}

const (
	NALU_RAW = iota
	NALU_AVCC
	NALU_ANNEXB
)

type NALUInfo struct {
	NALUStart int
	DataStart int
	End       int
	Data      []byte
}

func splitAVCC(b []byte) (out []NALUInfo, ok bool) {
	i := 0
	for {
		if len(b)-i < 4 {
			return
		}

		blen := pio.U32BE(b[i:])
		i += 4

		if blen > uint32(len(b)-i) {
			return
		}

		info := NALUInfo{
			NALUStart: i - 4,
			DataStart: i,
			End:       i + int(blen),
		}
		info.Data = b[info.DataStart:info.End]
		out = append(out, info)
		i = info.End

		if len(b)-i == 0 {
			ok = true
			return
		}
	}
}

func splitAnnexb(b []byte) (out []NALUInfo, ok bool) {
	// TODO:
	//
	// The pseudo logic from ISO/IEC 14496 - 10 Annex B:
	//
	//  byte_stream_nal_unit(NumBytesInNALunit) {
	//      while (next_bits(24) != 0x000001 &&
	//             next_bits(32) != 0x00000001)
	//          leading_zero_8bits /* equal to 0x00 */
	//
	//      if (next_bits(24) != 0x000001)
	//          zero_byte /* equal to 0x00 */
	//
	//      if (more_data_in_byte_stream()) {
	//          start_code_prefix_one_3bytes /* equal to 0x000001 */
	//          nal_unit(NumBytesInNALunit)
	//      }
	//
	//      while (more_data_in_byte_stream() &&
	//             next_bits(24) != 0x000001 &&
	//             next_bits(32) != 0x00000001)
	//          trailing_zero_8bits /* equal to 0x00 */
	//  }

	// Annex B
	const S = 0
	const S0 = 1
	const S00 = 2
	const S000 = 3
	nalustart := 0
	s := S
	from := 0
	cut := func(to, next int) {
		if from < to {
			info := NALUInfo{
				NALUStart: nalustart,
				DataStart: from,
				End:       to,
				Data:      b[from:to],
			}
			out = append(out, info)
		}
		from = next
	}
	for i, c := range b {
		switch c {
		case 0:
			if s < S000 {
				if s == S {
					nalustart = i
				}
				s++
			}
		case 1:
			switch s {
			case S00, S000:
				cut(i-s, i+1)
			}
			s = S
		default:
			s = S
		}
	}

	if from > 0 {
		cut(len(b), 0)
		ok = true
		return
	}

	return
}

func SplitNALUsRetInfo(b []byte) ([]NALUInfo, int) {
	if out, ok := splitAVCC(b); ok {
		return out, NALU_AVCC
	}
	if out, ok := splitAnnexb(b); ok {
		return out, NALU_ANNEXB
	}
	info := NALUInfo{NALUStart: 0, DataStart: 0, End: len(b), Data: b}
	return []NALUInfo{info}, NALU_RAW
}

func FillNALUsAVCC(nalus [][]byte) []byte {
	n := 0
	for _, nalu := range nalus {
		n += len(nalu) + 4
	}
	b := make([]byte, n)
	i := 0
	for _, nalu := range nalus {
		pio.WriteU32BE(b, &i, uint32(len(nalu)))
		pio.WriteBytes(b, &i, nalu)
	}
	return b
}

func FillNALUsAnnexb(nalus [][]byte) []byte {
	n := 0
	for _, nalu := range nalus {
		n += len(nalu) + len(StartCodeBytes)
	}
	b := make([]byte, n)
	i := 0
	for _, nalu := range nalus {
		pio.WriteBytes(b, &i, StartCodeBytes)
		pio.WriteBytes(b, &i, nalu)
	}
	return b
}

func SplitNALUs(b []byte) ([][]byte, int) {
	r := [][]byte{}
	infos, typ := SplitNALUsRetInfo(b)
	for _, info := range infos {
		r = append(r, info.Data)
	}
	return r, typ
}

func Map2arr(m map[int][]byte) (a [][]byte) {
	type elem struct {
		id int
		b  []byte
	}
	var e []elem
	for id, b := range m {
		e = append(e, elem{id, b})
	}
	sort.Slice(e, func(i, j int) bool {
		return e[i].id < e[j].id
	})
	for _, e := range e {
		a = append(a, e.b)
	}
	return
}

type Codec struct {
	ConfigBytes []byte
	SPS, PPS    map[int][]byte
	hash        []byte
	W, H        int
}

func (c Codec) Equal(b Codec) bool {
	return bytes.Compare(c.hash, b.hash) == 0
}

func (c Codec) ToConfig(b []byte, n *int) {
	firstSPS := []byte{}
	for _, s := range c.SPS {
		firstSPS = s
		break
	}

	var AVCProfileIndication byte
	var ProfileCompatibility byte
	var AVCLevelIndication byte
	if len(firstSPS) > 1 {
		AVCProfileIndication = firstSPS[1]
	}
	if len(firstSPS) > 2 {
		ProfileCompatibility = firstSPS[2]
	}
	if len(firstSPS) > 3 {
		AVCLevelIndication = firstSPS[3]
	}

	LengthSizeMinusOne := byte(0x3)
	spsCount := len(c.SPS)

	pio.WriteU8(b, n, 1)
	pio.WriteU8(b, n, AVCProfileIndication)
	pio.WriteU8(b, n, ProfileCompatibility)
	pio.WriteU8(b, n, AVCLevelIndication)
	pio.WriteU8(b, n, LengthSizeMinusOne|0xfc)
	pio.WriteU8(b, n, uint8(spsCount)|0xe0)

	for _, sps := range Map2arr(c.SPS) {
		pio.WriteU16BE(b, n, uint16(len(sps)))
		pio.WriteBytes(b, n, sps)
	}

	ppsCount := len(c.PPS)
	pio.WriteU8(b, n, uint8(ppsCount))

	for _, pps := range Map2arr(c.PPS) {
		pio.WriteU16BE(b, n, uint16(len(pps)))
		pio.WriteBytes(b, n, pps)
	}

	return
}

func FromDecoderConfig(b []byte) (c *Codec, err error) {
	nc := NewCodec()

	n := 0
	var v uint8

	for i := 0; i < 5; i++ {
		if _, err = pio.ReadU8(b, &n); err != nil {
			return
		}
	}

	if v, err = pio.ReadU8(b, &n); err != nil {
		return
	}
	spscount := int(v & 0x1f)

	for i := 0; i < spscount; i++ {
		var spslen uint16
		if spslen, err = pio.ReadU16BE(b, &n); err != nil {
			return
		}
		var sps []byte
		if sps, err = pio.ReadBytes(b, &n, int(spslen)); err != nil {
			return
		}
		nc.AddSPSPPS(sps)
	}

	if v, err = pio.ReadU8(b, &n); err != nil {
		return
	}
	ppscount := int(v)

	for i := 0; i < ppscount; i++ {
		var ppslen uint16
		if ppslen, err = pio.ReadU16BE(b, &n); err != nil {
			return
		}
		var pps []byte
		if pps, err = pio.ReadBytes(b, &n, int(ppslen)); err != nil {
			return
		}
		nc.AddSPSPPS(pps)
	}

	c = nc
	return
}

func NewCodec() (c *Codec) {
	return &Codec{
		SPS: map[int][]byte{},
		PPS: map[int][]byte{},
	}
}

func RemoveH264orH265EmulationBytes(b []byte) []byte {
	j := 0
	r := make([]byte, len(b))
	for i := 0; (i < len(b)) && (j < len(b)); {
		if i+2 < len(b) &&
			b[i] == 0 && b[i+1] == 0 && b[i+2] == 3 {
			r[j] = 0
			r[j+1] = 0
			j = j + 2
			i = i + 3
		} else {
			r[j] = b[i]
			j = j + 1
			i = i + 1
		}
	}
	return r[:j]
}

func ParseSPSId(data []byte) (id uint, err error) {
	data = RemoveH264orH265EmulationBytes(data)
	r := &bits.GolombBitReader{R: bytes.NewReader(data)}

	if _, err = r.ReadBits(8); err != nil {
		return
	}

	if _, err = r.ReadBits(8); err != nil {
		return
	}

	// constraint_set0_flag-constraint_set6_flag,reserved_zero_2bits
	if _, err = r.ReadBits(8); err != nil {
		return
	}

	// level_idc
	if _, err = r.ReadBits(8); err != nil {
		return
	}

	// seq_parameter_set_id
	if id, err = r.ReadExponentialGolombCode(); err != nil {
		return
	}

	return
}

type SPSInfo struct {
	Id                uint
	ProfileIdc        uint
	LevelIdc          uint
	ConstraintSetFlag uint

	MbWidth  uint
	MbHeight uint

	CropLeft   uint
	CropRight  uint
	CropTop    uint
	CropBottom uint

	Width  uint
	Height uint

	FPS uint
}

func ParseSPS(data []byte) (s SPSInfo, err error) {
	data = RemoveH264orH265EmulationBytes(data)
	r := &bits.GolombBitReader{R: bytes.NewReader(data)}

	if _, err = r.ReadBits(8); err != nil {
		return
	}

	if s.ProfileIdc, err = r.ReadBits(8); err != nil {
		return
	}

	// constraint_set0_flag-constraint_set6_flag,reserved_zero_2bits
	if s.ConstraintSetFlag, err = r.ReadBits(8); err != nil {
		return
	}
	s.ConstraintSetFlag = s.ConstraintSetFlag >> 2

	// level_idc
	if s.LevelIdc, err = r.ReadBits(8); err != nil {
		return
	}

	// seq_parameter_set_id
	if s.Id, err = r.ReadExponentialGolombCode(); err != nil {
		return
	}

	if s.ProfileIdc == 100 || s.ProfileIdc == 110 ||
		s.ProfileIdc == 122 || s.ProfileIdc == 244 ||
		s.ProfileIdc == 44 || s.ProfileIdc == 83 ||
		s.ProfileIdc == 86 || s.ProfileIdc == 118 {

		var chroma_format_idc uint
		if chroma_format_idc, err = r.ReadExponentialGolombCode(); err != nil {
			return
		}

		if chroma_format_idc == 3 {
			// residual_colour_transform_flag
			if _, err = r.ReadBit(); err != nil {
				return
			}
		}

		// bit_depth_luma_minus8
		if _, err = r.ReadExponentialGolombCode(); err != nil {
			return
		}
		// bit_depth_chroma_minus8
		if _, err = r.ReadExponentialGolombCode(); err != nil {
			return
		}
		// qpprime_y_zero_transform_bypass_flag
		if _, err = r.ReadBit(); err != nil {
			return
		}

		var seq_scaling_matrix_present_flag uint
		if seq_scaling_matrix_present_flag, err = r.ReadBit(); err != nil {
			return
		}

		if seq_scaling_matrix_present_flag != 0 {
			for i := 0; i < 8; i++ {
				var seq_scaling_list_present_flag uint
				if seq_scaling_list_present_flag, err = r.ReadBit(); err != nil {
					return
				}
				if seq_scaling_list_present_flag != 0 {
					var sizeOfScalingList uint
					if i < 6 {
						sizeOfScalingList = 16
					} else {
						sizeOfScalingList = 64
					}
					lastScale := uint(8)
					nextScale := uint(8)
					for j := uint(0); j < sizeOfScalingList; j++ {
						if nextScale != 0 {
							var delta_scale uint
							if delta_scale, err = r.ReadSE(); err != nil {
								return
							}
							nextScale = (lastScale + delta_scale + 256) % 256
						}
						if nextScale != 0 {
							lastScale = nextScale
						}
					}
				}
			}
		}
	}

	// log2_max_frame_num_minus4
	if _, err = r.ReadExponentialGolombCode(); err != nil {
		return
	}

	var pic_order_cnt_type uint
	if pic_order_cnt_type, err = r.ReadExponentialGolombCode(); err != nil {
		return
	}
	if pic_order_cnt_type == 0 {
		// log2_max_pic_order_cnt_lsb_minus4
		if _, err = r.ReadExponentialGolombCode(); err != nil {
			return
		}
	} else if pic_order_cnt_type == 1 {
		// delta_pic_order_always_zero_flag
		if _, err = r.ReadBit(); err != nil {
			return
		}
		// offset_for_non_ref_pic
		if _, err = r.ReadSE(); err != nil {
			return
		}
		// offset_for_top_to_bottom_field
		if _, err = r.ReadSE(); err != nil {
			return
		}
		var num_ref_frames_in_pic_order_cnt_cycle uint
		if num_ref_frames_in_pic_order_cnt_cycle, err = r.ReadExponentialGolombCode(); err != nil {
			return
		}
		for i := uint(0); i < num_ref_frames_in_pic_order_cnt_cycle; i++ {
			if _, err = r.ReadSE(); err != nil {
				return
			}
		}
	}

	// max_num_ref_frames
	if _, err = r.ReadExponentialGolombCode(); err != nil {
		return
	}

	// gaps_in_frame_num_value_allowed_flag
	if _, err = r.ReadBit(); err != nil {
		return
	}

	if s.MbWidth, err = r.ReadExponentialGolombCode(); err != nil {
		return
	}
	s.MbWidth++

	if s.MbHeight, err = r.ReadExponentialGolombCode(); err != nil {
		return
	}
	s.MbHeight++

	var frame_mbs_only_flag uint
	if frame_mbs_only_flag, err = r.ReadBit(); err != nil {
		return
	}
	if frame_mbs_only_flag == 0 {
		// mb_adaptive_frame_field_flag
		if _, err = r.ReadBit(); err != nil {
			return
		}
	}

	// direct_8x8_inference_flag
	if _, err = r.ReadBit(); err != nil {
		return
	}

	var frame_cropping_flag uint
	if frame_cropping_flag, err = r.ReadBit(); err != nil {
		return
	}
	if frame_cropping_flag != 0 {
		if s.CropLeft, err = r.ReadExponentialGolombCode(); err != nil {
			return
		}
		if s.CropRight, err = r.ReadExponentialGolombCode(); err != nil {
			return
		}
		if s.CropTop, err = r.ReadExponentialGolombCode(); err != nil {
			return
		}
		if s.CropBottom, err = r.ReadExponentialGolombCode(); err != nil {
			return
		}
	}

	s.Width = (s.MbWidth * 16) - s.CropLeft*2 - s.CropRight*2
	s.Height = ((2 - frame_mbs_only_flag) * s.MbHeight * 16) - s.CropTop*2 - s.CropBottom*2

	vui_parameter_present_flag, err := r.ReadBit()
	if err != nil {
		return
	}

	if vui_parameter_present_flag != 0 {
		aspect_ratio_info_present_flag, err := r.ReadBit()
		if err != nil {
			return s, err
		}

		if aspect_ratio_info_present_flag != 0 {
			aspect_ratio_idc, err := r.ReadBits(8)
			if err != nil {
				return s, err
			}

			if aspect_ratio_idc == 255 {
				sar_width, err := r.ReadBits(16)
				if err != nil {
					return s, err
				}
				sar_height, err := r.ReadBits(16)
				if err != nil {
					return s, err
				}

				_, _ = sar_width, sar_height
			}
		}

		overscan_info_present_flag, err := r.ReadBit()
		if err != nil {
			return s, err
		}

		if overscan_info_present_flag != 0 {
			overscan_appropriate_flagu, err := r.ReadBit()
			if err != nil {
				return s, err
			}

			_ = overscan_appropriate_flagu
		}
		video_signal_type_present_flag, err := r.ReadBit()
		if video_signal_type_present_flag != 0 {
			video_format, err := r.ReadBits(3)
			if err != nil {
				return s, err
			}
			_ = video_format
			video_full_range_flag, err := r.ReadBit()
			if err != nil {
				return s, err
			}
			_ = video_full_range_flag
			colour_description_present_flag, err := r.ReadBit()
			if err != nil {
				return s, err
			}
			if colour_description_present_flag != 0 {
				colour_primaries, err := r.ReadBits(8)
				if err != nil {
					return s, err
				}
				_ = colour_primaries
				transfer_characteristics, err := r.ReadBits(8)
				if err != nil {
					return s, err
				}
				_ = transfer_characteristics
				matrix_coefficients, err := r.ReadBits(8)
				if err != nil {
					return s, err
				}
				_ = matrix_coefficients
			}
		}
		chroma_loc_info_present_flag, err := r.ReadBit()
		if err != nil {
			return s, err
		}
		if chroma_loc_info_present_flag != 0 {
			chroma_sample_loc_type_top_field, err := r.ReadSE()
			if err != nil {
				return s, err
			}
			_ = chroma_sample_loc_type_top_field

			chroma_sample_loc_type_bottom_field, err := r.ReadSE()
			if err != nil {
				return s, err
			}

			_ = chroma_sample_loc_type_bottom_field
		}

		timing_info_present_flag, err := r.ReadBit()
		if err != nil {
			return s, err
		}

		if timing_info_present_flag != 0 {
			num_units_in_tick, err := r.ReadBits(32)
			if err != nil {
				return s, err
			}
			time_scale, err := r.ReadBits(32)
			if err != nil {
				return s, err
			}

			//num_units_in_tick/time_scale should yield field time  要求向下取整
			s.FPS = uint(math.Floor(float64(time_scale) / float64(num_units_in_tick) / 2.0))

			fixed_frame_rate_flag, err := r.ReadBit()
			if err != nil {
				return s, err
			}
			if fixed_frame_rate_flag != 0 {
				//utils.L.InfoLn("fixed_frame_rate_flag", fixed_frame_rate_flag)
				//have been devide 2
				//self.FPS = self.FPS / 2
			}
		}
	}

	return
}

type PPSInfo struct {
	Id    uint
	SPSId uint
}

func ParsePPS(data []byte) (self PPSInfo, err error) {
	r := &bits.GolombBitReader{R: bytes.NewReader(data)}

	if _, err = r.ReadBits(8); err != nil {
		return
	}

	// pic_parameter_set_id
	if self.Id, err = r.ReadExponentialGolombCode(); err != nil {
		return
	}

	// seq_parameter_set_id
	if self.SPSId, err = r.ReadExponentialGolombCode(); err != nil {
		return
	}

	return
}

func (c *Codec) AddSPSPPS(b []byte) {
	switch NALUType(b) {
	case NALU_SPS:
		if si, err := ParseSPS(b); err == nil {
			c.W = int(si.Width)
			c.H = int(si.Height)
			c.SPS[int(si.Id)] = append([]byte(nil), b...)
		}

	case NALU_PPS:
		if info, err := ParsePPS(b); err == nil {
			c.PPS[int(info.Id)] = append([]byte(nil), b...)
		}
	}

	h := sha1.New()
	for _, b := range Map2arr(c.SPS) {
		h.Write(b)
	}
	for _, b := range Map2arr(c.PPS) {
		h.Write(b)
	}
	c.hash = h.Sum(nil)
}

func FromOld(old Codec) *Codec {
	c := NewCodec()
	for _, sps := range old.SPS {
		c.AddSPSPPS(sps)
	}
	for _, pps := range old.PPS {
		c.AddSPSPPS(pps)
	}
	return c
}

func JoinNALUsAnnexb(nalus [][]byte) []byte {
	n := 0
	for _, nalu := range nalus {
		n += len(nalu) + len(StartCodeBytes)
	}
	b := make([]byte, n)
	i := 0
	for _, nalu := range nalus {
		pio.WriteBytes(b, &i, StartCodeBytes)
		pio.WriteBytes(b, &i, nalu)
	}
	return b
}

func JoinNALUsAVCC(nalus [][]byte) []byte {
	n := 0
	for _, nalu := range nalus {
		n += len(nalu) + 4
	}
	b := make([]byte, n)
	i := 0
	for _, nalu := range nalus {
		pio.WriteU32BE(b, &i, uint32(len(nalu)))
		pio.WriteBytes(b, &i, nalu)
	}
	return b
}
