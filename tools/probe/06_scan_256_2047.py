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


def rdx(addr,n):
    req=bytes([1,3])+struct.pack(">HH",addr,n); req+=crc(req)
    for _ in range(2):
        termios.tcflush(fd,termios.TCIOFLUSH); os.write(fd,req)
        buf=b""; end=time.time()+0.5; want=5+2*n
        while time.time()<end and len(buf)<want and not (len(buf)>=5 and buf[1]==0x83):
            if select.select([fd],[],[],0.03)[0]: buf+=os.read(fd,512)
        if len(buf)==want and buf[-2:]==crc(buf[:-2]) and buf[1]==3:
            return list(struct.unpack(">%dH"%n,buf[3:3+2*n]))
        if len(buf)==5 and buf[1]==0x83 and buf[-2:]==crc(buf[:-2]): return "exc"
    return None
vals={}; bad=[]
for st in range(256,2048,50):
    r=rdx(st,50)
    if isinstance(r,list):
        for j,v in enumerate(r): vals[st+j]=v
    else: bad.append((st,r))
print(len(vals),"regs read; failed chunks:",bad[:12],len(bad))
t={"SHrun":3828,"SHcyc":34920,"power":24266,"ign":34928,"att":35035}
for n,v in t.items():
    for k,x in vals.items():
        if x==v: print(n,v,"-> r",k)
nz={k:v for k,v in vals.items() if v}
print("nonzero count",len(nz)); print(list(nz.items())[:80])
