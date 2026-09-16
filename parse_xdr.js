const { xdr } = require('@stellar/stellar-sdk');

const base64Str = process.argv[2];
if (!base64Str) {
    console.log(JSON.stringify({ readBytes: 0, writeBytes: 0 }));
    process.exit(0);
}

try {
    const txData = xdr.SorobanTransactionData.fromXDR(base64Str, 'base64');
    
    // In newer stellar-sdk, properties might just be plain JS properties
    let resources;
    if (typeof txData.resources === 'function') {
        resources = txData.resources();
    } else {
        resources = txData.resources;
    }
    
    let readBytes = 0;
    let writeBytes = 0;
    let cpuInstructions = 0;
    
    if (resources) {
        if (typeof resources.diskReadBytes === 'function') {
            readBytes = resources.diskReadBytes();
        } else {
            readBytes = resources.diskReadBytes || 0;
        }
        
        if (typeof resources.writeBytes === 'function') {
            writeBytes = resources.writeBytes();
        } else {
            writeBytes = resources.writeBytes || 0;
        }
        
        if (typeof resources.instructions === 'function') {
            cpuInstructions = resources.instructions();
        } else {
            cpuInstructions = resources.instructions || 0;
        }
    }
    
    console.log(JSON.stringify({
        readBytes: Number(readBytes),
        writeBytes: Number(writeBytes),
        cpuInstructions: Number(cpuInstructions),
        memoryBytes: 0
    }));
} catch (e) {
    console.error("Error parsing XDR", e);
    process.exit(1);
}
