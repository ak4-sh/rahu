
def calculate_area(radius):
    pi = 3.14159
    if radius <= 0:
        return None
    
    area = pi * (radius ** 2)
    
    return area

items = [1, 2, "apple", True]
for i in items:
    if i == "apple":
        break
