-- force all existing feeds to be re-polled immediately so image_url gets populated
update feed_poll_jobs set scheduled_at = now();
