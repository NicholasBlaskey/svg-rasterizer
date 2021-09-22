# python3 fixTest1Svg.py > test1.svg

import re

f = open('test1.svg', 'r')
lines = f.readlines() #.split("\r\n")

for line in lines:
    if not "rect" in line:
        print(line)
    else:
        # Multiply x and y by 600
        x = float(line.split('x="')[-1].split('"')[0])
        y = float(line.split('y="')[-1].split('"')[0])

        line.replace("\r", "")
        print(line.split("x=")[0] +
              'x="%f" y ="%f" ' % (x * 500, y * 500) +
              "fill" + line.split("fill")[1].replace("0", "1")
        )
        
