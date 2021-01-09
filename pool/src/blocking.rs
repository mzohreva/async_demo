use std::error::Error;
use std::ops::{Deref, DerefMut};
use std::sync::{Arc, Mutex};

pub trait Connect: Sized {
    type Err: Error;

    fn connect() -> Result<Self, Self::Err>;
}

pub struct Pool<C> {
    pool: Arc<Mutex<Vec<C>>>,
}

impl<C: Connect> Pool<C> {
    pub fn new() -> Self {
        Pool { pool: Arc::new(Mutex::new(Vec::new())) }
    }

    pub fn get_connection(&self) -> Result<Lease<C>, C::Err> {
        let conn = match self.pool.lock().unwrap().pop() {
            Some(conn) => conn,
            None => C::connect()?,
        };
        Ok(Lease {
            conn: Some(conn),
            pool: self.pool.clone(),
        })
    }
}

pub struct Lease<C> {
    conn: Option<C>,
    pool: Arc<Mutex<Vec<C>>>,
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
            self.pool.lock().unwrap().push(conn);
        }
    }
}
