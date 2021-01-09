use crossbeam_channel as mpmc;
use std::error::Error;
use std::future::Future;
use std::ops::{Deref, DerefMut};
use std::pin::Pin;

pub trait Connect: Sized {
    type Err: Error;

    fn connect() -> Pin<Box<dyn Future<Output = Result<Self, Self::Err>> + Send>>;
}

pub struct Pool<C> {
    rx: mpmc::Receiver<C>,
    tx: mpmc::Sender<C>,
}

impl<C: Connect> Pool<C> {
    pub fn new() -> Self {
        let (tx, rx) = mpmc::unbounded();
        Pool { rx, tx }
    }

    pub async fn get_connection(&self) -> Result<Lease<C>, C::Err> {
        let conn = match self.rx.try_recv() {
            Ok(conn) => conn,
            Err(_) => C::connect().await?,
        };
        Ok(Lease {
            conn: Some(conn),
            tx: self.tx.clone(),
        })
    }
}

pub struct Lease<C> {
    conn: Option<C>,
    tx: mpmc::Sender<C>,
}

impl<C> Lease<C> {
    pub fn purge(mut self) {
        let _ = self.conn.take();
    }
}

impl<C> Deref for Lease<C> {
    type Target = C;

    fn deref(&self) -> &Self::Target {
        self.conn.as_ref().unwrap()
    }
}

impl<C> DerefMut for Lease<C> {
    fn deref_mut(&mut self) -> &mut Self::Target {
        self.conn.as_mut().unwrap()
    }
}

impl<C> Drop for Lease<C> {
    fn drop(&mut self) {
        if let Some(conn) = self.conn.take() {
            let _ = self.tx.send(conn);
        }
    }
}
