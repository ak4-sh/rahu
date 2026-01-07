def noop(x):
    if x == 0:
        return 0
    elif x > 0:
        return x
    else:
        return -x

def clamp(a, b, c):
    if a < b < c:
        return b
    elif a == b:
        return b + 0
    else:
        return c

def run(pairs, y):
    total = 0
    count = 0
    for x in range(pairs):
        if x > y > 3:
            print("branch_a")
            total = total + x
            count = count + 1
        elif x == y:
            print("branch_b")
            total = total + (x + 1)
            count = count + 1
        else:
            print("branch_c")
            total = total + 0
            count = count + 0

        total = total + -x
        total = total + +x

        if not x == 0:
            print("not_zero")
        else:
            print("is_zero")

    return total

def block_0000(pairs, y):
    acc = 0
    for x in range(pairs):
        if x > y > 3:
            print("b0000_a")
            acc = acc + x
        elif x == y:
            print("b0000_b")
            acc = acc + 1
        else:
            print("b0000_c")
            acc = acc + 0
        if not x == 0:
            acc = acc + 1
        else:
            acc = acc + 0
    return acc

def block_0001(pairs, y):
    acc = 0
    for x in range(pairs):
        if x > y > 3:
            print("b0001_a")
            acc = acc + x
        elif x == y:
            print("b0001_b")
            acc = acc + 1
        else:
            print("b0001_c")
            acc = acc + 0
        if not x == 0:
            acc = acc + 1
        else:
            acc = acc + 0
    return acc

def block_0002(pairs, y):
    acc = 0
    for x in range(pairs):
        if x > y > 3:
            print("b0002_a")
            acc = acc + x
        elif x == y:
            print("b0002_b")
            acc = acc + 1
        else:
            print("b0002_c")
            acc = acc + 0
        if not x == 0:
            acc = acc + 1
        else:
            acc = acc + 0
    return acc

def block_0003(pairs, y):
    acc = 0
    for x in range(pairs):
        if x > y > 3:
            print("b0003_a")
            acc = acc + x
        elif x == y:
            print("b0003_b")
            acc = acc + 1
        else:
            print("b0003_c")
            acc = acc + 0
        if not x == 0:
            acc = acc + 1
        else:
            acc = acc + 0
    return acc

def block_0004(pairs, y):
    acc = 0
    for x in range(pairs):
        if x > y > 3:
            print("b0004_a")
            acc = acc + x
        elif x == y:
            print("b0004_b")
            acc = acc + 1
        else:
            print("b0004_c")
            acc = acc + 0
        if not x == 0:
            acc = acc + 1
        else:
            acc = acc + 0
    return acc

def block_0005(pairs, y):
    acc = 0
    for x in range(pairs):
        if x > y > 3:
            print("b0005_a")
            acc = acc + x
        elif x == y:
            print("b0005_b")
            acc = acc + 1
        else:
            print("b0005_c")
            acc = acc + 0
        if not x == 0:
            acc = acc + 1
        else:
            acc = acc + 0
    return acc

def block_0006(pairs, y):
    acc = 0
    for x in range(pairs):
        if x > y > 3:
            print("b0006_a")
            acc = acc + x
        elif x == y:
            print("b0006_b")
            acc = acc + 1
        else:
            print("b0006_c")
            acc = acc + 0
        if not x == 0:
            acc = acc + 1
        else:
            acc = acc + 0
    return acc

def block_0007(pairs, y):
    acc = 0
    for x in range(pairs):
        if x > y > 3:
            print("b0007_a")
            acc = acc + x
        elif x == y:
            print("b0007_b")
            acc = acc + 1
        else:
            print("b0007_c")
            acc = acc + 0
        if not x == 0:
            acc = acc + 1
        else:
            acc = acc + 0
    return acc

def block_0008(pairs, y):
    acc = 0
    for x in range(pairs):
        if x > y > 3:
            print("b0008_a")
            acc = acc + x
        elif x == y:
            print("b0008_b")
            acc = acc + 1
        else:
            print("b0008_c")
            acc = acc + 0
        if not x == 0:
            acc = acc + 1
        else:
            acc = acc + 0
    return acc

def block_0009(pairs, y):
    acc = 0
    for x in range(pairs):
        if x > y > 3:
            print("b0009_a")
            acc = acc + x
        elif x == y:
            print("b0009_b")
            acc = acc + 1
        else:
            print("b0009_c")
            acc = acc + 0
        if not x == 0:
            acc = acc + 1
        else:
            acc = acc + 0
    return acc

def block_0010(pairs, y):
    acc = 0
    for x in range(pairs):
        if x > y > 3:
            print("b0010_a")
            acc = acc + x
        elif x == y:
            print("b0010_b")
            acc = acc + 1
        else:
            print("b0010_c")
            acc = acc + 0
        if not x == 0:
            acc = acc + 1
        else:
            acc = acc + 0
    return acc

def block_0011(pairs, y):
    acc = 0
    for x in range(pairs):
        if x > y > 3:
            print("b0011_a")
            acc = acc + x
        elif x == y:
            print("b0011_b")
            acc = acc + 1
        else:
            print("b0011_c")
            acc = acc + 0
        if not x == 0:
            acc = acc + 1
        else:
            acc = acc + 0
    return acc

def block_0012(pairs, y):
    acc = 0
    for x in range(pairs):
        if x > y > 3:
            print("b0012_a")
            acc = acc + x
        elif x == y:
            print("b0012_b")
            acc = acc + 1
        else:
            print("b0012_c")
            acc = acc + 0
        if not x == 0:
            acc = acc + 1
        else:
            acc = acc + 0
    return acc

def block_0013(pairs, y):
    acc = 0
    for x in range(pairs):
        if x > y > 3:
            print("b0013_a")
            acc = acc + x
        elif x == y:
            print("b0013_b")
            acc = acc + 1
        else:
            print("b0013_c")
            acc = acc + 0
        if not x == 0:
            acc = acc + 1
        else:
            acc = acc + 0
    return acc

def block_0014(pairs, y):
    acc = 0
    for x in range(pairs):
        if x > y > 3:
            print("b0014_a")
            acc = acc + x
        elif x == y:
            print("b0014_b")
            acc = acc + 1
        else:
            print("b0014_c")
            acc = acc + 0
        if not x == 0:
            acc = acc + 1
        else:
            acc = acc + 0
    return acc

def block_0015(pairs, y):
    acc = 0
    for x in range(pairs):
        if x > y > 3:
            print("b0015_a")
            acc = acc + x
        elif x == y:
            print("b0015_b")
            acc = acc + 1
        else:
            print("b0015_c")
            acc = acc + 0
        if not x == 0:
            acc = acc + 1
        else:
            acc = acc + 0
    return acc

def block_0016(pairs, y):
    acc = 0
    for x in range(pairs):
        if x > y > 3:
            print("b0016_a")
            acc = acc + x
        elif x == y:
            print("b0016_b")
            acc = acc + 1
        else:
            print("b0016_c")
            acc = acc + 0
        if not x == 0:
            acc = acc + 1
        else:
            acc = acc + 0
    return acc

def block_0017(pairs, y):
    acc = 0
    for x in range(pairs):
        if x > y > 3:
            print("b0017_a")
            acc = acc + x
        elif x == y:
            print("b0017_b")
            acc = acc + 1
        else:
            print("b0017_c")
            acc = acc + 0
        if not x == 0:
            acc = acc + 1
        else:
            acc = acc + 0
    return acc

def block_0018(pairs, y):
    acc = 0
    for x in range(pairs):
        if x > y > 3:
            print("b0018_a")
            acc = acc + x
        elif x == y:
            print("b0018_b")
            acc = acc + 1
        else:
            print("b0018_c")
            acc = acc + 0
        if not x == 0:
            acc = acc + 1
        else:
            acc = acc + 0
    return acc

def block_0019(pairs, y):
    acc = 0
    for x in range(pairs):
        if x > y > 3:
            print("b0019_a")
            acc = acc + x
        elif x == y:
            print("b0019_b")
            acc = acc + 1
        else:
            print("b0019_c")
            acc = acc + 0
        if not x == 0:
            acc = acc + 1
        else:
            acc = acc + 0
    return acc

def block_0020(pairs, y):
    acc = 0
    for x in range(pairs):
        if x > y > 3:
            print("b0020_a")
            acc = acc + x
        elif x == y:
            print("b0020_b")
            acc = acc + 1
        else:
            print("b0020_c")
            acc = acc + 0
        if not x == 0:
            acc = acc + 1
        else:
            acc = acc + 0
    return acc

def block_0021(pairs, y):
    acc = 0
    for x in range(pairs):
        if x > y > 3:
            print("b0021_a")
            acc = acc + x
        elif x == y:
            print("b0021_b")
            acc = acc + 1
        else:
            print("b0021_c")
            acc = acc + 0
        if not x == 0:
            acc = acc + 1
        else:
            acc = acc + 0
    return acc

def block_0022(pairs, y):
    acc = 0
    for x in range(pairs):
        if x > y > 3:
            print("b0022_a")
            acc = acc + x
        elif x == y:
            print("b0022_b")
            acc = acc + 1
        else:
            print("b0022_c")
            acc = acc + 0
        if not x == 0:
            acc = acc + 1
        else:
            acc = acc + 0
    return acc

def block_0023(pairs, y):
    acc = 0
    for x in range(pairs):
        if x > y > 3:
            print("b0023_a")
            acc = acc + x
        elif x == y:
            print("b0023_b")
            acc = acc + 1
        else:
            print("b0023_c")
            acc = acc + 0
        if not x == 0:
            acc = acc + 1
        else:
            acc = acc + 0
    return acc

def block_0024(pairs, y):
    acc = 0
    for x in range(pairs):
        if x > y > 3:
            print("b0024_a")
            acc = acc + x
        elif x == y:
            print("b0024_b")
            acc = acc + 1
        else:
            print("b0024_c")
            acc = acc + 0
        if not x == 0:
            acc = acc + 1
        else:
            acc = acc + 0
    return acc

def block_0025(pairs, y):
    acc = 0
    for x in range(pairs):
        if x > y > 3:
            print("b0025_a")
            acc = acc + x
        elif x == y:
            print("b0025_b")
            acc = acc + 1
        else:
            print("b0025_c")
            acc = acc + 0
        if not x == 0:
            acc = acc + 1
        else:
            acc = acc + 0
    return acc

def block_0026(pairs, y):
    acc = 0
    for x in range(pairs):
        if x > y > 3:
            print("b0026_a")
            acc = acc + x
        elif x == y:
            print("b0026_b")
            acc = acc + 1
        else:
            print("b0026_c")
            acc = acc + 0
        if not x == 0:
            acc = acc + 1
        else:
            acc = acc + 0
    return acc

def block_0027(pairs, y):
    acc = 0
    for x in range(pairs):
        if x > y > 3:
            print("b0027_a")
            acc = acc + x
        elif x == y:
            print("b0027_b")
            acc = acc + 1
        else:
            print("b0027_c")
            acc = acc + 0
        if not x == 0:
            acc = acc + 1
        else:
            acc = acc + 0
    return acc

def block_0028(pairs, y):
    acc = 0
    for x in range(pairs):
        if x > y > 3:
            print("b0028_a")
            acc = acc + x
        elif x == y:
            print("b0028_b")
            acc = acc + 1
        else:
            print("b0028_c")
            acc = acc + 0
        if not x == 0:
            acc = acc + 1
        else:
            acc = acc + 0
    return acc

def block_0029(pairs, y):
    acc = 0
    for x in range(pairs):
        if x > y > 3:
            print("b0029_a")
            acc = acc + x
        elif x == y:
            print("b0029_b")
            acc = acc + 1
        else:
            print("b0029_c")
            acc = acc + 0
        if not x == 0:
            acc = acc + 1
        else:
            acc = acc + 0
    return acc

def main():
    pairs = 200
    y = 5
    print(run(pairs, y))
    print(block_0000(pairs, y))
    print(block_0001(pairs, y))
    print(block_0002(pairs, y))
    print(block_0003(pairs, y))
    print(block_0004(pairs, y))
    print(block_0005(pairs, y))
    print(block_0006(pairs, y))
    print(block_0007(pairs, y))
    print(block_0008(pairs, y))
    print(block_0009(pairs, y))
    print(block_0010(pairs, y))
    print(block_0011(pairs, y))
    print(block_0012(pairs, y))
    print(block_0013(pairs, y))
    print(block_0014(pairs, y))
    print(block_0015(pairs, y))
    print(block_0016(pairs, y))
    print(block_0017(pairs, y))
    print(block_0018(pairs, y))
    print(block_0019(pairs, y))
    print(block_0020(pairs, y))
    print(block_0021(pairs, y))
    print(block_0022(pairs, y))
    print(block_0023(pairs, y))
    print(block_0024(pairs, y))
    print(block_0025(pairs, y))
    print(block_0026(pairs, y))
    print(block_0027(pairs, y))
    print(block_0028(pairs, y))
    print(block_0029(pairs, y))
    print(noop(1))
    print(clamp(1, 2, 3))

main()
