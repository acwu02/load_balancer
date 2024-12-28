import requests

URL = 'http://localhost:80/'

def load_test(num_requests):
    for i in range(num_requests):
        response = requests.get(URL)
        print(response.text)

if __name__ == '__main__':
    load_test(100)

