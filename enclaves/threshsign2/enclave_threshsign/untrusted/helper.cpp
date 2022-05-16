#include <string>
#include <exception>

#include "helper.h"
#include "constants.h"

libff::alt_bn128_Fq HashToFq(
    std::shared_ptr<std::array<uint8_t, 32>> hash_byte_arr)
{
    libff::bigint<libff::alt_bn128_q_limbs> from_hex;

    std::vector<uint8_t> hex(64);
    for (size_t i = 0; i < 32; ++i)
    {
        hex[2 * i] = static_cast<int>(hash_byte_arr->at(i)) / 16;
        hex[2 * i + 1] = static_cast<int>(hash_byte_arr->at(i)) % 16;
    }
    mpn_set_str(from_hex.data, hex.data(), 64, 16);

    libff::alt_bn128_Fq ret_val(from_hex);

    return ret_val;
}

libff::alt_bn128_G1 HashtoG1(std::shared_ptr<std::array<uint8_t, 32>> hash_byte_arr)
{
    libff::alt_bn128_Fq x1(HashToFq(hash_byte_arr));

    libff::alt_bn128_G1 result;

    while (true)
    {
        libff::alt_bn128_Fq y1_sqr = x1 ^ 3;
        y1_sqr = y1_sqr + libff::alt_bn128_coeff_b;

        libff::alt_bn128_Fq euler = y1_sqr ^ libff::alt_bn128_Fq::euler;

        if (euler == libff::alt_bn128_Fq::one() ||
            euler == libff::alt_bn128_Fq::zero())
        { // if y1_sqr is a square
            result.X = x1;
            libff::alt_bn128_Fq temp_y = y1_sqr.sqrt();

            mpz_t pos_y;
            mpz_init(pos_y);

            temp_y.as_bigint().to_mpz(pos_y);

            mpz_t neg_y;
            mpz_init(neg_y);

            (-temp_y).as_bigint().to_mpz(neg_y);

            if (mpz_cmp(pos_y, neg_y) < 0)
            {
                temp_y = -temp_y;
            }

            mpz_clear(pos_y);
            mpz_clear(neg_y);

            result.Y = temp_y;
            break;
        }
        else
        {
            x1 = x1 + 1;
        }
    }
    result.Z = libff::alt_bn128_Fq::one();

    return result;
}

std::string *stringFromFq(libff::alt_bn128_Fq *_fq)
{
    std::string *ret = nullptr;
    mpz_t t;
    mpz_init(t);
    SAFE_CHAR_BUF(arr, BUF_LEN);

    try {
        _fq->as_bigint().to_mpz(t);
        char *tmp = mpz_get_str(arr, 10, t);
        ret = new std::string(tmp);
    } catch (std::exception &e) {
        std::cout << "error in stringFromFq" << e.what();
        goto clean;
    } catch (...) {
        std::cout << "Unknown throwable in stringFromFq";
        goto clean;
    }

    clean:
    mpz_clear(t);
    return ret;
}

std::string *stringFromG1(libff::alt_bn128_G1 *_g1)
{
    CHECK_STATE(_g1);

    std::string *sX = nullptr;
    std::string *sY = nullptr;
    std::string *ret = nullptr;

    try
    {
        _g1->to_affine_coordinates();

        auto sX = stringFromFq(&_g1->X);

        if (!sX)
        {
            goto clean;
        }

        auto sY = stringFromFq(&_g1->Y);

        if (!sY)
        {
            goto clean;
        }

        ret = new std::string(*sX + ":" + *sY);
    }
    catch (std::exception &e)
    {
        goto clean;
    }
    catch (...)
    {
        goto clean;
    }

clean:

    SAFE_FREE(sX);
    SAFE_FREE(sY);

    return ret;
}

std::shared_ptr< std::vector< std::string > > SplitString(
    std::shared_ptr< std::string > str, const std::string& delim ) {
    std::vector< std::string > tokens;
    size_t prev = 0, pos = 0;
    do {
        pos = str->find( delim, prev );
        if ( pos == std::string::npos )
            pos = str->length();
        std::string token = str->substr( prev, pos - prev );
        if ( !token.empty() )
            tokens.push_back( token );
        prev = pos + delim.length();
    } while ( pos < str->length() && prev < str->length() );

    return std::make_shared< std::vector< std::string > >( tokens );
}