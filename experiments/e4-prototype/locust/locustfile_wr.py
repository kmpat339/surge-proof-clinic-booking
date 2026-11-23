import gevent
from locust import FastHttpUser, task, between
from locustfile import FlashCrowd

class WaitingPatient(FastHttpUser):
    wait_time = between(0.5, 1.5)

    @task
    def book_through_waiting_room(self):
        # join the line and get ticket
        ticket = self.client.post("/join").json()["ticket"]

        # check every 0.5s until admitted
        while not self.client.get(f"/status?ticket={ticket}", name="/status").json()["admitted"]:
            gevent.sleep(0.5)

        # book once admitted
        self.client.post("/book")