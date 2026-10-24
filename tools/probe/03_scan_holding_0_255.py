import os, termios, time, select, glob, struct
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
rows=[]
for addr in range(256):
    req=bytes([1,3])+struct.pack(">HH",addr,1); req+=crc(req)
    for attempt in range(3):
        termios.tcflush(fd,termios.TCIOFLUSH); os.write(fd,req)
        buf=b""; end=time.time()+0.3
        while time.time()<end:
            if select.select([fd],[],[],0.03)[0]: buf+=os.read(fd,256)
            if len(buf)>=7 and buf[1]==3 or len(buf)>=5 and buf[1]==0x83: break
        if buf[-2:]==crc(buf[:-2]) and len(buf)>=5: break
        buf=b""
    if len(buf)==7 and buf[1]==3:
        v=struct.unpack(">H",buf[3:5])[0]; s=struct.unpack(">h",buf[3:5])[0]
        rows.append((addr,"ok",v,s))
    elif len(buf)==5 and buf[1]==0x83: rows.append((addr,f"exc{buf[2]}","",""))
    else: rows.append((addr,"timeout","",""))
with open("holding_registers_slave1.csv","w") as f:
    f.write("addr,status,unsigned,signed\n")
    for r in rows: f.write(",".join(map(str,r))+"\n")
ok=[r for r in rows if r[1]=="ok"]
print(len(ok),"readable;",sum(r[1].startswith("exc") for r in rows),"exceptions;",sum(r[1]=="timeout" for r in rows),"timeouts")
for r in ok: print(f"{r[0]:4d} (0x{r[0]:04x}) = {r[2]:5d} (0x{r[2]:04x}) signed {r[3]}")
