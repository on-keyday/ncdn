#!/bin/bash

export MSYS_NO_PATHCONV=1

# Arguments
CA_NUMBER=$1
# specify the unique number if you want to overwrite the key
UNIQUE_NUMBER=$2
# can overwrite the CA
SHOULD_CREATE_CA=$3

if [ -z "$SHOULD_CREATE_CA" ]; then
    SHOULD_CREATE_CA=false
fi

if [ -z "$UNIQUE_NUMBER" ]; then
    UNIQUE_NUMBER=$(date +%Y%m%d%H%M%S)
fi

# Set OpenSSL Command
OSSL=openssl

# Configuration Files
CONFIG_DIR=./caconfig
ROOT_CA_CONFIG=$CONFIG_DIR/root_ca.conf
INTERMEDIATE_CA_CONFIG=$CONFIG_DIR/intermediate_ca.conf

# Output Directories
CA_DIR=ca
export ROOT_CA_DIR=$CA_DIR/root_ca
export INTERMEDIATE_CA_DIR=$CA_DIR/intermediate_ca

if [ -z "$TARGET_HOSTNAME" ]; then
    TARGET_HOSTNAME=localhost
fi
export TARGET_HOSTNAME

SERVER_NUMBER=$UNIQUE_NUMBER



if [ -z "$CA_NUMBER" ]; then
    CA_NUMBER=$UNIQUE_NUMBER
elif [ ! $SHOULD_CREATE_CA ]; then
    if [ ! -d $CA_DIR/private/$CA_NUMBER ]; then
        echo "CA Number $CA_NUMBER does not exist."
        exit 1
    fi
    if [ ! -d $CA_DIR/certs/$CA_NUMBER ]; then
        echo "CA Number $CA_NUMBER does not exist."
        exit 1
    fi
fi

CA_KEY_DIR=$CA_DIR/private/$CA_NUMBER
CA_CERT_DIR=$CA_DIR/certs/$CA_NUMBER

SERVER_KEY_DIR=$CA_DIR/private/$SERVER_NUMBER
SERVER_CERT_DIR=$CA_DIR/certs/$SERVER_NUMBER


# Create Directories
if [ ! -d $CA_DIR ]; then
    mkdir $CA_DIR
fi

if [ ! -d $ROOT_CA_DIR ]; then
    ./make_ca_dir.sh $ROOT_CA_DIR
fi

if [ ! -d $INTERMEDIATE_CA_DIR ]; then
    ./make_ca_dir.sh $INTERMEDIATE_CA_DIR
fi

if [ ! -d $CA_KEY_DIR ]; then
    mkdir -p $CA_KEY_DIR
fi

if [ ! -d $CA_CERT_DIR ]; then
    mkdir -p $CA_CERT_DIR
fi

if [ ! -d $SERVER_KEY_DIR ]; then
    mkdir -p $SERVER_KEY_DIR
fi

if [ ! -d $SERVER_CERT_DIR ]; then
    mkdir -p $SERVER_CERT_DIR
fi

# Output Files


ROOT_CA_KEY=$CA_KEY_DIR/root_ca.key
ROOT_CA_CERT=$CA_CERT_DIR/root_ca.crt
ROOT_CA_CSR=$CA_KEY_DIR/root_ca.csr

INTERMEDIATE_CA_KEY=$CA_KEY_DIR/intermediate_ca.key
INTERMEDIATE_CA_CERT=$CA_CERT_DIR/intermediate_ca.crt
INTERMEDIATE_CA_CSR=$CA_KEY_DIR/intermediate_ca.csr

SERVER_KEY=$SERVER_KEY_DIR/server.key
SERVER_CERT=$SERVER_CERT_DIR/server.crt
SERVER_CSR=$SERVER_KEY_DIR/server.csr

CLIENT_KEY=$SERVER_KEY_DIR/client.key
CLIENT_CERT=$SERVER_CERT_DIR/client.crt
CLIENT_CSR=$SERVER_KEY_DIR/client.csr

# Subjects

SUBJECT_COMMON_PREFIX="/C=JP/ST=Unknown/L=Mt.Fuji/O=Team ($UNIQUE_NUMBER)"
ROOT_CA_SUBJECT="$SUBJECT_COMMON_PREFIX/OU=Root CA/CN=Root CA"
INTERMEDIATE_CA_SUBJECT="$SUBJECT_COMMON_PREFIX/OU=Intermediate CA/CN=Intermediate CA"
SERVER_SUBJECT="$SUBJECT_COMMON_PREFIX/OU=Server/CN=localhost"
CLIENT_SUBJECT="$SUBJECT_COMMON_PREFIX/OU=Client/CN=browser"

# Days
ROOT_CA_DAYS=3650
INTERMEDIATE_CA_DAYS=3650
SERVER_DAYS=3650
CLIENT_DAYS=3650

# Extensions
ROOT_CA_EXTENSIONS=v3_ca
INTERMEDIATE_CA_EXTENSIONS=v3_ca
SERVER_EXTENSIONS=v3_server
CLIENT_EXTENSIONS=v3_client

KEY_ALGORITHM=ed25519

if [ "$HOMESERVER_KEY_ALGORITHM" != "" ]; then
    KEY_ALGORITHM=$HOMESERVER_KEY_ALGORITHM
fi

# Create CA Files if not exists
if [ ! -f $ROOT_CA_KEY ]; then
# Create Root CA
    # Create EdDSA Private Key
    #$OSSL ecparam -genkey -name prime256v1 -out $ROOT_CA_KEY.tmp
    $OSSL genpkey -algorithm "$KEY_ALGORITHM" -out $ROOT_CA_KEY.tmp
    # convert private key to PKCS8
    $OSSL pkcs8 -topk8 -nocrypt -in $ROOT_CA_KEY.tmp -out $ROOT_CA_KEY
    rm $ROOT_CA_KEY.tmp
    # Create root certificate
    $OSSL req -new -sha256\
        -key $ROOT_CA_KEY\
        -config $ROOT_CA_CONFIG\
        -out $ROOT_CA_CERT\
        -subj "$ROOT_CA_SUBJECT"\
        -x509\
        -extensions $ROOT_CA_EXTENSIONS\
        -days $ROOT_CA_DAYS
fi
# Create Intermediate CA Files if not exists
if [ ! -f $INTERMEDIATE_CA_KEY ]; then
# Create Intermediate CA
    # Create private key
    $OSSL genpkey -algorithm "$KEY_ALGORITHM" -out $INTERMEDIATE_CA_KEY.tmp
    # convert private key to PKCS8
    $OSSL pkcs8 -topk8 -nocrypt -in $INTERMEDIATE_CA_KEY.tmp -out $INTERMEDIATE_CA_KEY
    rm $INTERMEDIATE_CA_KEY.tmp
    # Create Certificate Signing Request
    $OSSL req -new -sha256\
        -key $INTERMEDIATE_CA_KEY\
        -config $INTERMEDIATE_CA_CONFIG\
        -out $INTERMEDIATE_CA_CSR\
        -subj "$INTERMEDIATE_CA_SUBJECT"

    # Sign Intermediate CA with Root CA
    $OSSL ca -batch\
        -config $ROOT_CA_CONFIG\
        -cert $ROOT_CA_CERT\
        -keyfile $ROOT_CA_KEY\
        -in $INTERMEDIATE_CA_CSR\
        -out $INTERMEDIATE_CA_CERT\
        -extensions $INTERMEDIATE_CA_EXTENSIONS\
        -days $INTERMEDIATE_CA_DAYS\
        -policy policy_anything    
fi

# Create Server Certificate
    # Create private key
    $OSSL genpkey -algorithm "$KEY_ALGORITHM" -out $SERVER_KEY.tmp
    # convert private key to PKCS8
    $OSSL pkcs8 -topk8 -nocrypt -in $SERVER_KEY.tmp -out $SERVER_KEY
    rm $SERVER_KEY.tmp
    # Create Certificate Signing Request
    $OSSL req -new -sha256\
        -key $SERVER_KEY\
        -out $SERVER_CSR\
        -subj "$SERVER_SUBJECT"
    # Sign Server Certificate with Intermediate CA
    $OSSL ca -batch\
        -config $INTERMEDIATE_CA_CONFIG\
        -cert $INTERMEDIATE_CA_CERT\
        -keyfile $INTERMEDIATE_CA_KEY\
        -in $SERVER_CSR\
        -out $SERVER_CERT\
        -extensions $SERVER_EXTENSIONS\
        -days $SERVER_DAYS\
        -policy policy_anything

# Create Client Certificate
  # Create private key
    $OSSL genpkey -algorithm "$KEY_ALGORITHM" -out $CLIENT_KEY.tmp
    # convert private key to PKCS8
    $OSSL pkcs8 -topk8 -nocrypt -in $CLIENT_KEY.tmp -out $CLIENT_KEY
    rm $CLIENT_KEY.tmp
    # Create Certificate Signing Request
    $OSSL req -new -sha256\
        -key $CLIENT_KEY\
        -out $CLIENT_CSR\
        -subj "$CLIENT_SUBJECT"
    # Sign Client Certificate with Intermediate CA
    $OSSL ca -batch\
        -config $INTERMEDIATE_CA_CONFIG\
        -cert $INTERMEDIATE_CA_CERT\
        -keyfile $INTERMEDIATE_CA_KEY\
        -in $CLIENT_CSR\
        -out $CLIENT_CERT\
        -extensions $CLIENT_EXTENSIONS\
        -days $CLIENT_DAYS\
        -policy policy_anything
    # Export Client Certificate to PKCS12
    $OSSL pkcs12 -export\
        -in $CLIENT_CERT\
        -inkey $CLIENT_KEY\
        -out $CLIENT_CERT.p12\
        -name "Brgen Client Certificate"\
        -passout pass:$UNIQUE_NUMBER
        
# append intermediate ca to server cert
cat $INTERMEDIATE_CA_CERT >> $SERVER_CERT

unset MSYS_NO_PATHCONV
unset ROOT_CA_DIR
unset INTERMEDIATE_CA_DIR