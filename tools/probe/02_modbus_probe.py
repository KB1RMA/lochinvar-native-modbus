import os, termios, time, select, sys, glob
dev = glob.glob("/dev/cu.usbserial*")[0]
def crc(d):
    c=0xFFFF
    for x in d:
        c^=x
        for _ in range(8): c=(c>>1)^0xA001 if c&1 else c>>1
    return bytes([c&255,c>>8])
fd = os.open(dev, os.O_RDWR|os.O_NOCTTY|os.O_NONBLOCK)
a = termios.tcgetattr(fd)
hits=0
for parity in ("N","E"):
  for b in [9600,19200,38400,57600,115200,4800,2400]:
    a[0]=0;a[1]=0;a[3]=0
    c=termios.CS8|termios.CREAD|termios.CLOCAL|(termios.PARENB if parity=="E" else 0)
    a[2]=c;a[4]=a[5]=getattr(termios,f"B{b}");a[6][termios.VMIN]=0;a[6][termios.VTIME]=0
    termios.tcsetattr(fd,termios.TCSANOW,a)
    for sid in list(range(1,17))+[247]:
      for fn in (3,4):
        req=bytes([sid,fn,0,0,0,1]); req+=crc(req)
        termios.tcflush(fd,termios.TCIOFLUSH); os.write(fd,req)
        buf=b""; end=time.time()+0.25
        while time.time()<end:
            if select.select([fd],[],[],0.05)[0]: buf+=os.read(fd,256)
        # strip local echo of our own request
        resp = buf[len(req):] if buf.startswith(req) else buf
        if buf:
            hits+=1; print(f"{b} 8{parity}1 id={sid} fn={fn}: rx={buf.hex(' ')} {'(echo only)' if not resp else ''}"); sys.stdout.flush()
print("done, responses:",hits)
