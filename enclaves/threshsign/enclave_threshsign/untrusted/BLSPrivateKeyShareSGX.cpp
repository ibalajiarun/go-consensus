/*
  Copyright (C) 2018-2019 SKALE Labs

  This file is part of libBLS.

  libBLS is free software: you can redistribute it and/or modify
  it under the terms of the GNU Affero General Public License as published
  by the Free Software Foundation, either version 3 of the License, or
  (at your option) any later version.

  libBLS is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU Affero General Public License for more details.

  You should have received a copy of the GNU Affero General Public License
  along with libBLS.  If not, see <https://www.gnu.org/licenses/>.

  @file BLSPrivateKeyShare.cpp
  @author Stan Kladko, Sveta Rogova
  @date 2019
*/

// #include "BLSSigShare.h"
// #include "BLSSignature.h"
#include "BLSutils.h"

#include "sgx_urts.h"
#include "Enclave_u.h"

using namespace std;

#include <stdlib.h>
#include <iostream>
#include <map>
#include <memory>

#include "BLSPrivateKeyShareSGX.h"

std::string *stringFromFq(libff::alt_bn128_Fq *_fq)
{

  mpz_t t;
  mpz_init(t);

  _fq->as_bigint().to_mpz(t);

  char arr[mpz_sizeinbase(t, 10) + 2];

  char *tmp = mpz_get_str(arr, 10, t);
  mpz_clear(t);

  return new std::string(tmp);
}

std::string *stringFromG1(libff::alt_bn128_G1 *_g1)
{

  auto sX = stringFromFq(&_g1->X);
  auto sY = stringFromFq(&_g1->Y);
  auto sZ = stringFromFq(&_g1->Z);

  auto sG1 = new std::string(*sX + ":" + *sY + ":" + *sZ);

  delete (sX);
  delete (sY);
  delete (sZ);

  return sG1;
}

BLSPrivateKeyShareSGX::BLSPrivateKeyShareSGX(sgx_enclave_id_t _eid,
                                             size_t _requiredSigners,
                                             size_t _totalSigners)
{
  eid = _eid;
  requiredSigners = _requiredSigners;
  totalSigners = _totalSigners;

  // std::cerr << "ENTER BLSPrivateKeyShareSGX CONSTRUCTOR" << std::endl;

  if (requiredSigners > totalSigners)
  {

    throw std::invalid_argument("requiredSigners > totalSigners");
  }

  if (totalSigners == 0)
  {
    throw std::invalid_argument("totalSigners == 0");
  }
}

std::string BLSPrivateKeyShareSGX::signWithHelperSGXstr(
    std::shared_ptr<std::array<uint8_t, 32>> hash_byte_arr,
    size_t _signerIndex)
{
  shared_ptr<signatures::Bls> obj;

  //  if (_signerIndex == 0) {
  //    BOOST_THROW_EXCEPTION(runtime_error("Zero signer index"));
  //  }
  if (hash_byte_arr == nullptr)
  {
    std::cerr << "Hash is null" << std::endl;
    BOOST_THROW_EXCEPTION(runtime_error("Hash is null"));
  }

  obj = make_shared<signatures::Bls>(
      signatures::Bls(requiredSigners, totalSigners));

  std::pair<libff::alt_bn128_G1, std::string> hash_with_hint =
      obj->HashtoG1withHint(hash_byte_arr);

  int errStatus = 0;

  string *xStr = stringFromFq(&(hash_with_hint.first.X));

  if (xStr == nullptr)
  {
    std::cerr << "Null xStr" << std::endl;
    BOOST_THROW_EXCEPTION(runtime_error("Null xStr"));
  }

  string *yStr = stringFromFq(&(hash_with_hint.first.Y));

  if (yStr == nullptr)
  {
    std::cerr << "Null yStr" << std::endl;
    BOOST_THROW_EXCEPTION(runtime_error("Null yStr"));
  }

  char errMsg[BUF_LEN];
  memset(errMsg, 0, BUF_LEN);

  char xStrArg[BUF_LEN];
  char yStrArg[BUF_LEN];
  char signature[BUF_LEN];

  memset(xStrArg, 0, BUF_LEN);
  memset(yStrArg, 0, BUF_LEN);

  strncpy(xStrArg, xStr->c_str(), BUF_LEN);
  strncpy(yStrArg, yStr->c_str(), BUF_LEN);

  // size_t sz = 0;

  // uint8_t encryptedKey[BUF_LEN];

  // bool result = hex2carray(encryptedKeyHex->c_str(), &sz, encryptedKey);

  // if (!result) {
  //   cerr <<   "Invalid hex encrypted key" << endl;
  //   BOOST_THROW_EXCEPTION(std::invalid_argument("Invalid hex encrypted key"));
  // }

  // cerr << "Key is " + *encryptedKeyHex << endl;

  // cerr << "Before SGX call ecall_bls_sign" << endl;

  sgx_status_t status =
      ecall_bls_sign(eid, &errStatus, errMsg, xStrArg, yStrArg, signature);

  // strncpy(signature, "8175162913343900215959836578795929492705714455632345516427532159927644835012:15265825550804683171644566522808807137117748565649051208189914766494241035855", 1024);

  // printf("sig is: %s\n", signature);

  if (status != SGX_SUCCESS)
  {
    gmp_printf("SGX enclave call  to bls_sign_message failed: 0x%04x\n", status);
    BOOST_THROW_EXCEPTION(runtime_error("SGX enclave call  to bls_sign_message failed"));
  }

  if (errStatus != 0)
  {
    BOOST_THROW_EXCEPTION(runtime_error("Enclave bls_sign_message failed:" + to_string(errStatus) + ":" + errMsg));
    return nullptr;
  }

  int sigLen;

  if ((sigLen = strnlen(signature, 10)) < 10)
  {
    BOOST_THROW_EXCEPTION(runtime_error("Signature is too short:" + to_string(sigLen)));
  }

  std::string hint = BLSutils::ConvertToString(hash_with_hint.first.Y) + ":" +
                     hash_with_hint.second;

  std::string sig = signature;

  sig.append(":");
  sig.append(hint);

  return sig;
}

std::shared_ptr<BLSSigShare> BLSPrivateKeyShareSGX::signWithHelperSGX(
    std::shared_ptr<std::array<uint8_t, 32>> hash_byte_arr,
    size_t _signerIndex)
{
  /*  shared_ptr<signatures::Bls> obj;

  if (_signerIndex == 0) {
    BOOST_THROW_EXCEPTION(runtime_error("Zero signer index"));
  }
  if (hash_byte_arr == nullptr) {
    BOOST_THROW_EXCEPTION(runtime_error("Hash is null"));
  }

  obj = make_shared<signatures::Bls>(
      signatures::Bls(requiredSigners, totalSigners));

  std::pair<libff::alt_bn128_G1, std::string> hash_with_hint =
      obj->HashtoG1withHint(hash_byte_arr);

  int errStatus = 0;


  string* xStr = stringFromFq(&(hash_with_hint.first.X));

  if (xStr == nullptr) {
    BOOST_THROW_EXCEPTION(runtime_error("Null xStr"));
  }

  string* yStr = stringFromFq(&(hash_with_hint.first.Y));

  if (xStr == nullptr) {
    BOOST_THROW_EXCEPTION(runtime_error("Null yStr"));
  }


  char errMsg[BUF_LEN];
  memset(errMsg, 0, BUF_LEN);

  char xStrArg[BUF_LEN];
  char yStrArg[BUF_LEN];
  char signature [BUF_LEN];

  memset(xStrArg, 0, BUF_LEN);
  memset(yStrArg, 0, BUF_LEN);

  strncpy(xStrArg, xStr->c_str(), BUF_LEN);
  strncpy(yStrArg, yStr->c_str(), BUF_LEN);

  size_t sz = 0;


  uint8_t encryptedKey[BUF_LEN];

  bool result = hex2carray(encryptedKeyHex->c_str(), &sz, encryptedKey);

  if (!result) {
    BOOST_THROW_EXCEPTION(std::invalid_argument("Invalid hex encrypted key"));
  }

  cerr << "Key is " + *encryptedKeyHex << endl;

//  sgx_status_t status =
//      bls_sign_message(eid, &errStatus, errMsg, encryptedKey,
//                       encryptedKeyHex->size() / 2, xStrArg, yStrArg, signature);

  strncpy(signature, "8175162913343900215959836578795929492705714455632345516427532159927644835012:15265825550804683171644566522808807137117748565649051208189914766494241035855", 1024);

  printf("---: %s\n", signature);


//  if (status != SGX_SUCCESS) {
//    gmp_printf("SGX enclave call  to bls_sign_message failed: 0x%04x\n", status);
//    BOOST_THROW_EXCEPTION(runtime_error("SGX enclave call  to bls_sign_message failed"));
//  }


//  if (errStatus != 0) {
//    BOOST_THROW_EXCEPTION(runtime_error("Enclave bls_sign_message failed:" + to_string(errStatus) + ":" + errMsg ));
//    return nullptr;
//  }

  int sigLen;

  if ((sigLen = strnlen(signature, 10)) < 10) {
    BOOST_THROW_EXCEPTION(runtime_error("Signature too short:" + to_string(sigLen)));
  }




  std::string hint = BLSutils::ConvertToString(hash_with_hint.first.Y) + ":" +
                     hash_with_hint.second;

  auto sig = make_shared<string>(signature);

  sig->append(":");
  sig->append(hint);*/

  std::string signature = signWithHelperSGXstr(hash_byte_arr, _signerIndex);

  auto sig = make_shared<string>(signature);

  //BLSSigShare* sig_test = new BLSSigShare(sig, _signerIndex, requiredSigners, totalSigners);

  //std::string hello = "hello";
  //std::cout << "HINT " << *((void**)&(sig_test->hint)) << std::endl;

  //std::shared_ptr<BLSSigShare> s; s.reset( sig_test );//(sig, _signerIndex, requiredSigners,
  //totalSigners);

  std::shared_ptr<BLSSigShare> s = std::make_shared<BLSSigShare>(sig, _signerIndex, requiredSigners,
                                                                 totalSigners);

  return s;
}

std::shared_ptr<std::vector<std::shared_ptr<BLSSigShare>>> BLSPrivateKeyShareSGX::batchSignWithHelperSGX(
    std::shared_ptr<std::vector<std::shared_ptr<std::array<uint8_t, 32>>>> hash_byte_arrs,
    size_t _signerIndex)
{
  std::vector<std::string> signatures = batchSignWithHelperSGXstr(hash_byte_arrs, _signerIndex);

  auto sigshares = std::make_shared<std::vector<std::shared_ptr<BLSSigShare>>>();
  sigshares->reserve(hash_byte_arrs->size());
  for (auto signature : signatures)
  {
    auto sig = make_shared<string>(signature);

    // std::shared_ptr<BLSSigShare> s = std::make_shared<BLSSigShare>(sig, _signerIndex, requiredSigners,
    //  totalSigners);
    sigshares->emplace_back(std::make_shared<BLSSigShare>(sig, _signerIndex, requiredSigners, totalSigners));
  }

  return sigshares;
}

std::vector<std::string> BLSPrivateKeyShareSGX::batchSignWithHelperSGXstr(
    std::shared_ptr<std::vector<std::shared_ptr<std::array<uint8_t, 32>>>> hash_byte_arrs,
    size_t _signerIndex)
{
  size_t count = hash_byte_arrs->size();

  int errStatus = 0;

  char errMsg[BUF_LEN];
  memset(errMsg, 0, BUF_LEN);

  char *xStrArg = (char *)calloc(BUF_LEN * count, 1);
  char *yStrArg = (char *)calloc(BUF_LEN * count, 1);
  char *signature = (char *)calloc(BUF_LEN * count, 1);

  // char xStrArg[BUF_LEN*count];
  // char yStrArg[BUF_LEN*count];
  // char signature[BUF_LEN*count];

  shared_ptr<signatures::Bls> obj = make_shared<signatures::Bls>(
      signatures::Bls(requiredSigners, totalSigners));

  vector<std::pair<libff::alt_bn128_G1, std::string>> hash_with_hints;
  hash_with_hints.reserve(count);

  for (size_t i = 0; i < count; i++)
  {
    auto hash_byte_arr = hash_byte_arrs->at(i);
    if (hash_byte_arr == nullptr)
    {
      std::cerr << "Hash is null" << std::endl;
      BOOST_THROW_EXCEPTION(runtime_error("Hash is null"));
    }

    std::pair<libff::alt_bn128_G1, std::string> hash_with_hint =
        obj->HashtoG1withHint(hash_byte_arr);

    hash_with_hints.emplace_back(hash_with_hint);

    string *xStr = stringFromFq(&(hash_with_hint.first.X));

    if (xStr == nullptr)
    {
      std::cerr << "Null xStr" << std::endl;
      BOOST_THROW_EXCEPTION(runtime_error("Null xStr"));
    }

    string *yStr = stringFromFq(&(hash_with_hint.first.Y));

    if (yStr == nullptr)
    {
      std::cerr << "Null yStr" << std::endl;
      BOOST_THROW_EXCEPTION(runtime_error("Null yStr"));
    }

    // memset(xStrArg[i*1024], 0, BUF_LEN);
    // memset(yStrArg[i*1024], 0, BUF_LEN);

    strncpy(&xStrArg[i * BUF_LEN], xStr->c_str(), BUF_LEN);
    strncpy(&yStrArg[i * BUF_LEN], yStr->c_str(), BUF_LEN);
  }

  // size_t sz = 0;
  // cerr << "Before SGX call ecall_bls_sign" << endl;

  sgx_status_t status =
      ecall_bls_batch_sign(eid, &errStatus, errMsg, count, xStrArg, yStrArg, signature);

  free(xStrArg);
  free(yStrArg);

  if (status != SGX_SUCCESS)
  {
    gmp_printf("SGX enclave call  to bls_sign_message failed: 0x%04x\n", status);
    std::ostringstream stringStream;
    stringStream << "SGX enclave call  to bls_sign_message failed: " << status;
    BOOST_THROW_EXCEPTION(runtime_error(stringStream.str()));
  }

  if (errStatus != 0)
  {
    BOOST_THROW_EXCEPTION(runtime_error("Enclave bls_sign_message failed:" + to_string(errStatus) + ":" + errMsg));
    // return nullptr;
  }

  std::vector<std::string> sigs;
  sigs.reserve(count);
  for (size_t i = 0; i < count; i++)
  {
    size_t sigLen;
    auto hash_with_hint = hash_with_hints.at(i);

    if ((sigLen = strnlen(&signature[i * BUF_LEN], 10)) < 10)
    {
      BOOST_THROW_EXCEPTION(runtime_error("Signature is too short:" + to_string(sigLen)));
    }

    std::string hint = BLSutils::ConvertToString(hash_with_hint.first.Y) + ":" +
                       hash_with_hint.second;

    std::string sig = &signature[i * BUF_LEN];

    sig.append(":");
    sig.append(hint);

    sigs.emplace_back(sig);
  }
  free(signature);

  return sigs;
}