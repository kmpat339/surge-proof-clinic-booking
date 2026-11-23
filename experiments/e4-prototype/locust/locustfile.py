from locust import FastHttpUser, LoadTestShape, task, between

class Patient(FastHttpUser):
    # each user tries to book once per second
    wait_time = between(0.5, 1.5)

    @task
    def book(self):
        # each user tries to book
        self.client.post("/book")

class FlashCrowd(LoadTestShape):
    # end time in seconds, number of users, users added per second
    # first is normal traffic, second is when vaccine opens and crowd arrives, third is when rush fades
    stages = [
        (30, 20, 10), 
        (90, 300, 100),
        (150, 20, 100),
    ]
    # locust calls tick to see how many users shuold be running and how fast to add them
    # this controls the locust web UI for us!
    def tick(self):
        run_time = self.get_run_time()
        for end, users, spawn_rate in self.stages:
            if run_time < end:
                return users, spawn_rate
        # stop test
        return None
