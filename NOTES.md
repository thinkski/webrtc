* There are two STUN binding requests -- first, one from caller to callee,
  followed by one from callee to caller. Both occur on same port tuple.

* The message integrity check on the STUN binding request uses the ice-pwd
  value verbatim as the HMAC-SHA1 key. The length (byte 3, zero indexed) must
  be changed prior to computing the HMAC. The length is the number of bytes
  in the STUN message _after_ the header, which is 20 (0x14) bytes. For the
  HMAC computating, the length must be set to include the message integrity
  check, but the actual bytes over which the HMAC is computed do not include
  the message integrity check bytes.

* DTLS Client Hello must be sent from and to same port tuple as STUN binding
  request and response.

## Domain ideas

RTC Logic (rtclogic.com)

RTC Ware (rtcware.com)

uRTC (micrortc.com)

Artcy
Artci
seertc