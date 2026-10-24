import os, termios, time, select, sys, glob
dev = glob.glob("/dev/cu.usbserial*")[0]
bauds = [9600,19200,38400,57600,115200,4800,2400,1200]
fd = os.open(dev, os.O_RDWR|os.O_NOCTTY|os.O_NONBLOCK)
a = termios.tcgetattr(fd)
for parity in ("N","E","O"):
  for b in bauds:
    a[0]=0; a[1]=0; a[3]=0
    c = termios.CS8|termios.CREAD|termios.CLOCAL
    if parity!="N": c |= termios.PARENB
    if parity=="O": c |= termios.PARODD
    a[2]=c; a[4]=a[5]=getattr(termios,f"B{b}"); a[6][termios.VMIN]=0; a[6][termios.VTIME]=0
    try:
        termios.tcsetattr(fd, termios.TCSANOW, a); termios.tcflush(fd, termios.TCIOFLUSH)
    except Exception as e:
        print(b,parity,"fail",e); continue
    buf=b""; end=time.time()+3
    while time.time()<end:
        if select.select([fd],[],[],0.2)[0]: buf+=os.read(fd,4096)
    print(f"{b} 8{parity}1: {len(buf)} bytes", buf[:48].hex(' ')); sys.stdout.flush()
os.close(fd)
