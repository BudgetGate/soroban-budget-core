const { xdr } = require('@stellar/stellar-sdk');
const base64Str = process.argv[2];
const txData = xdr.SorobanTransactionData.fromXDR(base64Str, 'base64');
console.log(txData);
