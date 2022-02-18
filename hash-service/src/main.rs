use sha2::{Digest, Sha256};
use std::io::Result;
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::{TcpListener, TcpStream};

async fn handle_client(mut socket: TcpStream) -> Result<()> {
    loop {
        // read a length-encoded input buffer
        let len = socket.read_u16_le().await?;
        if len == 0 {
            break;
        }
        let mut buf = vec![0u8; len as usize];
        socket.read_exact(&mut buf).await?;
        // compute hash of the input
        let result = Sha256::digest(&buf);
        // send back the result
        socket.write_all(result.as_slice()).await?;
    }
    Ok(())
}

#[tokio::main]
async fn main() -> Result<()> {
    let addr = "127.0.0.1:34567";

    let listener = TcpListener::bind(addr).await?;
    println!("Hash service listening on: {}", listener.local_addr()?);

    loop {
        let (socket, addr) = listener.accept().await?;
        tokio::spawn(async move {
            match handle_client(socket).await {
                Err(e) => println!("Error handling request from client {}: {}", addr, e),
                Ok(()) => {}
            }
        });
    }
}
