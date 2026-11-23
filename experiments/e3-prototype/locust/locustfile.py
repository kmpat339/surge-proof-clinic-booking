from locust import FastHttpUser, task, between

class Browser(FastHttpUser):
    # each simulated user waits 0.5 to 1.5 seconds between requests 
    wait_time = between(0.5, 1.5)

    @task
    def list_slots(self):
        self.client.get("/slots")