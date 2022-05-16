#ifndef UNTRUSTED_HELPER_H
#define UNTRUSTED_HELPER_H

#include <memory>
#include <vector>
#include <string>
// #include <polycrypto/PolyCrypto.h>

#include <libff/algebra/curves/alt_bn128/alt_bn128_pp.hpp>

#define SAFE_FREE(__X__) if (__X__) {free(__X__); __X__ = NULL;}
#define SAFE_DELETE(__X__) if (__X__) {delete(__X__); __X__ = NULL;}
#define SAFE_CHAR_BUF(__X__, __Y__)  ;char __X__ [ __Y__ ]; memset(__X__, 0, __Y__);
#define CHECK_STATE(_EXPRESSION_) \
    if (!(_EXPRESSION_)) { \
        auto __msg__ = std::string("State check failed::") + #_EXPRESSION_ +  " " + std::string(__FILE__) + ":" + std::to_string(__LINE__); \
        throw std::runtime_error(__msg__);}

libff::alt_bn128_Fq HashToFq( std::shared_ptr< std::array< uint8_t, 32 > > );
libff::alt_bn128_G1 HashtoG1( std::shared_ptr< std::array< uint8_t, 32 > > );

std::string *stringFromFq(libff::alt_bn128_Fq*);
std::string *stringFromG1(libff::alt_bn128_G1*);
std::shared_ptr< std::vector< std::string > > SplitString(std::shared_ptr< std::string >, const std::string&);

#endif