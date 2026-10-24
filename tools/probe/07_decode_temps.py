import os, termios, time, select, glob, struct, sys
dev = glob.glob("/dev/cu.usbserial*")[0]
def crc(d):
    c=0xFFFF
    for x in d:
        c^=x
        for _ in range(8): c=(c>>1)^0xA001 if c&1 else c>>1
    return struct.pack("<H",c)
fd=os.open(dev,os.O_RDWR|os.O_NOCTTY|os.O_NONBLOCK)
a=termios.tcgetattr(fd); a[0]=a[1]=a[3]=0
a[2]=termios.CS8|termios.CREAD|termios.CLOCAL; a[4]=a[5]=termios.B19200
a[6][termios.VMIN]=0;a[6][termios.VTIME]=0
termios.tcsetattr(fd,termios.TCSANOW,a)
def rd(addr,n):
    req=bytes([1,3])+struct.pack(">HH",addr,n); req+=crc(req)
    for _ in range(3):
        termios.tcflush(fd,termios.TCIOFLUSH); os.write(fd,req)
        buf=b""; end=time.time()+0.6; want=5+2*n
        while time.time()<end and len(buf)<want:
            if select.select([fd],[],[],0.03)[0]: buf+=os.read(fd,512)
        if len(buf)==want and buf[-2:]==crc(buf[:-2]) and buf[1]==3:
            return list(struct.unpack(">%dH"%n,buf[3:3+2*n]))
    return None
def snap():
    out=[]
    for s in range(0,200,50):
        r=rd(s,50)
        if r is None: return None
        out+=r
    return out

c=snap()
print(time.strftime("%H:%M:%S"))
f=lambda v: 168+((v-32768)-13735)/26.2
for k,v in enumerate(c):
    if v&0x8000 and v!=0x8000: print(f"r{k}={v} -> {f(v):.1f}F  (15bit {v-32768})")
print("r80",c[80],"r110",c[110],"r151",c[151])
