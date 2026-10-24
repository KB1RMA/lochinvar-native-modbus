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
first=snap(); t0=time.time(); changed={}
prev=first
for i in range(20):
    time.sleep(2)
    cur=snap()
    if not cur: continue
    for k,(x,y) in enumerate(zip(prev,cur)):
        if x!=y: changed.setdefault(k,[x]).append(y)
    prev=cur
def temp(v): return f"{(v&0x7fff)/100:.2f}"
print("reg  first  last   (as 0.01 w/o 0x8000 bit)  changes")
for k in range(200):
    if first[k] in (0,) and k not in changed: continue
    if k in range(30,45): continue
    print(f"{k:3d} {first[k]:6d} {prev[k]:6d}  0x{first[k]:04x}  {temp(first[k]) if first[k]&0x8000 else ''}  {'CHANGED '+str(changed[k][:6]) if k in changed else ''}")
