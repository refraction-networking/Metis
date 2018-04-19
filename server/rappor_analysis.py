# -*- coding: utf-8 -*-
"""
Created on Fri Mar 30 12:14:23 2018

@author: Audrey
"""

# Copyright 2014 Google Inc. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

#NOTE (Audrey): To run this code, use git bash, which is running python 2.7 for some 
#reason. Don't use Spyder, it's on 3.6.

"""RAPPOR client library.
Privacy is ensured without a third party by only sending RAPPOR'd data over the
network (as opposed to raw client data).
Note that we use MD5 for the Bloom filter hash function. (security not required)
"""
import csv
import hashlib
import hmac
import json
import struct
import sys
import numpy as np
import time

from random import SystemRandom
from sklearn import linear_model

class Error(Exception):
  pass


def log(msg, *args):
  if args:
    msg = msg % args
  print >>sys.stderr, msg


class Params(object):
  """RAPPOR encoding parameters.
  These affect privacy/anonymity.  See the paper for details.
  """
  def __init__(self, num_bloombits=16, num_hashes=2, num_cohorts=64, 
               prob_p=0.50, prob_q=0.75, prob_f=0.50):
    self.num_bloombits = num_bloombits      # Number of bloom filter bits (k)
    self.num_hashes = num_hashes          # Number of bloom filter hashes (h)
    self.num_cohorts = num_cohorts        # Number of cohorts (m) was 64
    self.prob_p = prob_p           # Probability p
    self.prob_q = prob_q           # Probability q
    self.prob_f = prob_f           # Probability f

  # For testing
  def __eq__(self, other):
    return self.__dict__ == other.__dict__

  def __repr__(self):
    return repr(self.__dict__)

  def to_json(self):
    """Convert this instance to JSON.
    The names are be compatible with the apps/api server.
    """
    return json.dumps({
        'numBits': self.num_bloombits,
        'numHashes': self.num_hashes,
        'numCohorts': self.num_cohorts,
        'probPrr': self.prob_f,
        'probIrr0': self.prob_p,
        'probIrr1': self.prob_q,
    })

  # NOTE:
  # - from_csv is currently used in sum_bits.py
  # - to_csv is in rappor_sim.print_params
  @staticmethod
  def from_csv(f):
    """Read the RAPPOR parameters from a CSV file.
    Args:
      f: file handle
    Returns:
      Params instance.
    Raises:
      rappor.Error: when the file is malformed.
    """
    c = csv.reader(f)
    ok = False
    p = Params()
    for i, row in enumerate(c):

      if i == 0:
        if row != ['k', 'h', 'm', 'p', 'q', 'f']:
          raise Error('Header %s is malformed; expected k,h,m,p,q,f' % row)

      elif i == 1:
        try:
          # NOTE: May raise exceptions
          p.num_bloombits = int(row[0])
          p.num_hashes = int(row[1])
          p.num_cohorts = int(row[2])
          p.prob_p = float(row[3])
          p.prob_q = float(row[4])
          p.prob_f = float(row[5])
        except (ValueError, IndexError) as e:
          raise Error('Row is malformed: %s' % e)
        ok = True

      else:
        raise Error('Params file should only have two rows')

    if not ok:
      raise Error("Expected second row with params")

    return p


class _SecureRandom(object):
  """Returns an integer where each bit has probability p of being 1."""

  def __init__(self, prob_one, num_bits):
    self.prob_one = prob_one
    self.num_bits = num_bits

  def __call__(self):
    p = self.prob_one
    rand = SystemRandom()
    r = 0

    for i in xrange(self.num_bits):
      bit = rand.random() < p
      r |= (bit << i)  # using bool as int
    return r


class SecureIrrRand(object):
  """Python's os.random()"""

  def __init__(self, params):
    """
    Args:
      params: rappor.Params
    """
    num_bits = params.num_bloombits
    # IRR probabilities

    self.p_gen = _SecureRandom(params.prob_p, num_bits)
    self.q_gen = _SecureRandom(params.prob_q, num_bits)


def to_big_endian(i):
  """Convert an integer to a 4 byte big endian string.  Used for hashing."""
  # https://docs.python.org/2/library/struct.html
  # - Big Endian (>) for consistent network byte order.
  # - L means 4 bytes when using >
  return struct.pack('>L', i)


def get_bloom_bits(word, cohort, num_hashes, num_bloombits):
  """Return an array of bits to set in the bloom filter.
  In the real report, we bitwise-OR them together.  In hash candidates, we put
  them in separate entries in the "map" matrix.
  """
  value = to_big_endian(cohort) + word  # Cohort is 4 byte prefix.
  md5 = hashlib.md5(value)

  digest = md5.digest()

  # Each has is a byte, which means we could have up to 256 bit Bloom filters.
  # There are 16 bytes in an MD5, in which case we can have up to 16 hash
  # functions per Bloom filter.
  if num_hashes > len(digest):
    raise RuntimeError("Can't have more than %d hashes" % md5)

  #log('hash_input %r', value)
  #log('Cohort %d', cohort)
  #log('MD5 %s', md5.hexdigest())

  return [ord(digest[i]) % num_bloombits for i in xrange(num_hashes)]


def get_prr_masks(secret, word, prob_f, num_bits):
  h = hmac.new(secret, word, digestmod=hashlib.sha256)
  #log('word %s, secret %s, HMAC-SHA256 %s', word, secret, h.hexdigest())

  # Now go through each byte
  digest_bytes = h.digest()
  assert len(digest_bytes) == 32

  # Use 32 bits.  If we want 64 bits, it may be fine to generate another 32
  # bytes by repeated HMAC.  For arbitrary numbers of bytes it's probably
  # better to use the HMAC-DRBG algorithm.
  if num_bits > len(digest_bytes):
    raise RuntimeError('%d bits is more than the max of %d', num_bits, len(d))

  threshold128 = prob_f * 128

  uniform = 0
  f_mask = 0

  for i in xrange(num_bits):
    ch = digest_bytes[i]
    byte = ord(ch)

    u_bit = byte & 0x01  # 1 bit of entropy
    uniform |= (u_bit << i)  # maybe set bit in mask

    rand128 = byte >> 1  # 7 bits of entropy
    noise_bit = (rand128 < threshold128)
    f_mask |= (noise_bit << i)  # maybe set bit in mask

  return uniform, f_mask


def bit_string(irr, num_bloombits):
  """Like bin(), but uses leading zeroes, and no '0b'."""
  s = ''
  bits = []
  for bit_num in xrange(num_bloombits):
    if irr & (1 << bit_num):
      bits.append('1')
    else:
      bits.append('0')
  return ''.join(reversed(bits))


class Encoder(object):
  """Obfuscates values for a given user using the RAPPOR privacy algorithm."""

  def __init__(self, params, cohort, secret, irr_rand):
    """
    Args:
      params: RAPPOR Params() controlling privacy
      cohort: integer cohort, for Bloom hashing.
      secret: secret string, for the PRR to be a deterministic function of the
        reported value.
      irr_rand: IRR randomness interface.
    """
    # RAPPOR params.  NOTE: num_cohorts isn't used.  p and q are used by
    # irr_rand.
    self.params = params
    self.cohort = cohort  # associated: MD5
    self.secret = secret  # associated: HMAC-SHA256
    self.irr_rand = irr_rand  # p and q used

  def _internal_encode_bits(self, bits):
    """Helper function for simulation / testing.
    Returns:
      The PRR and IRR.  The PRR should never be sent over the network.
    """
    # Compute Permanent Randomized Response (PRR).
    uniform, f_mask = get_prr_masks(
        self.secret, to_big_endian(bits), self.params.prob_f,
        self.params.num_bloombits)

    # Suppose bit i of the Bloom filter is B_i.  Then bit i of the PRR is
    # defined as:
    #
    # 1   with prob f/2
    # 0   with prob f/2
    # B_i with prob 1-f

    # Uniform bits are 1 with probability 1/2, and f_mask bits are 1 with
    # probability f.  So in the expression below:
    #
    # - Bits in (uniform & f_mask) are 1 with probability f/2.
    # - (bloom_bits & ~f_mask) clears a bloom filter bit with probability
    # f, so we get B_i with probability 1-f.
    # - The remaining bits are 0, with remaining probability f/2.

    prr = (bits & ~f_mask) | (uniform & f_mask)

    #log('U %s / F %s', bit_string(uniform, num_bits),
    #    bit_string(f_mask, num_bits))

    #log('B %s / PRR %s', bit_string(bloom_bits, num_bits),
    #    bit_string(prr, num_bits))

    # Compute Instantaneous Randomized Response (IRR).
    # If PRR bit is 0, IRR bit is 1 with probability p.
    # If PRR bit is 1, IRR bit is 1 with probability q.
    p_bits = self.irr_rand.p_gen()
    q_bits = self.irr_rand.q_gen()

    irr = (p_bits & ~prr) | (q_bits & prr)

    return prr, irr  # IRR is the rappor

  def _internal_encode(self, word):
    """Helper function for simulation / testing.
    Returns:
      The Bloom filter bits, PRR, and IRR.  The first two values should never
      be sent over the network.
    """
    bloom_bits = get_bloom_bits(word, self.cohort, self.params.num_hashes,
                                self.params.num_bloombits)

    bloom = 0
    for bit_to_set in bloom_bits:
      bloom |= (1 << bit_to_set)

    prr, irr = self._internal_encode_bits(bloom)
    return bloom, prr, irr

  def encode_bits(self, bits):
    """Encode a string with RAPPOR.
    Args:
      bits: An integer representing bits to encode.
    Returns:
      An integer that is the IRR (Instantaneous Randomized Response).
    """
    _, irr = self._internal_encode_bits(bits)
    return irr

  def encode(self, word):
    """Encode a string with RAPPOR.
    Args:
      word: the string that should be privately transmitted.
    Returns:
      An integer that is the IRR (Instantaneous Randomized Response).
    """
    _, _, irr = self._internal_encode(word)
    return irr

p = Params()
s = SecureIrrRand(p)
e = Encoder(p, 1, "my secret string", s)

'''results = []
for i in range(0, 100):
    results.append(e.encode(bytearray("www.google.com", 'utf-8')))
#print(results)

domains = ["google.com", "facebook.com", "baidu.com", "twitter.com", "bongacams.com"]
blooms = []
for domain in domains:
    bloom, _, _ = e._internal_encode(domain)
    blooms.append(bloom)
print(blooms)
'''

def generateReports(params, domains, numDuplicates):
    s = SecureIrrRand(p)
    reports = {}
    for cohort in range(0, params.num_cohorts):
        e = Encoder(params, cohort, "my secret string", s)
        reportsPerCohort = []
        for i in range(0, numDuplicates):
            for d in domains:
                reportsPerCohort.append(e.encode(d))
        reports[cohort] = reportsPerCohort
    return reports

def estimateSetBits(reports, params):
    """
    Estimate which bits were truly set in B for a particular cohort.
    
    Returns: Y, a slice of the number of times each bit was estimated
    to have been truly set in B.
    
    This function will be called for each cohort. j represents the jth
    cohort and is included only to keep variable names matching the paper.
    """
    p = params.prob_p
    q = params.prob_q
    f = params.prob_f
    Nj = len(reports)
    Y_j = []
    for i in range(0,params.num_bloombits):
        c_ij = 0.0
        for rep in reports:
            ithBit = (rep & (1<<i))>>i
            #print("Rep: ", "{0:b}".format(rep), ", ith bit: ", ithBit)
            if ithBit == 1:
                c_ij += 1.0
        #print("C_ij is: ", c_ij)
        t_ij = (c_ij-(p+0.5*f*q-0.5*f*p)*Nj)/((1-f)*(q-p))
        Y_j.append(t_ij)
    return Y_j

def getDomains(numDomains):
    try:
        domainFile = open('alexa_top.txt', 'r')
    except:
        print("Couldn't open file")
    domains = []

    for line in domainFile:
        #print(line.rstrip())
        domains.append(line.rstrip())
    return domains[0:numDomains]

def getBloomFilters(domains, numCohorts, numHashes, numBloomBits):
    blooms = {}
    for m in range(0, numCohorts):
        bloomsForCohort = []
        for d in domains:
            bits = get_bloom_bits(d, m, numHashes, numBloomBits)
            #print("Cohort ", m, ": Bits for ", d, " are: ", bits)
            bloomsForCohort.append(bits)
        blooms[m] = bloomsForCohort
    #print("Blooms:")
    #print(blooms)
    return blooms

def makeDesignMatrix(params, numDomains):
    k = params.num_bloombits #number of bits in Bloom filter
    m = params.num_cohorts
    #M is number of candidate strings
    #h is the number of hash functions per cohort
    domains = getDomains(numDomains)
    blooms = getBloomFilters(domains, params.num_cohorts, params.num_hashes, params.num_bloombits)
    
    X = np.zeros([k*m, numDomains])
    
    for cohort in blooms:
        #print("Cohort: ", cohort)
        domainColumn = 0
        for domain in blooms[cohort]:
            #domain is a list of the bits that need to be set
            for bitToSet in domain:
                X[cohort*k+bitToSet, domainColumn] = 1
            domainColumn+=1
    #print(X)
    return X, domains

def doLassoRegression(params, numDomains, reports):
    X, domains = makeDesignMatrix(params, numDomains)
    Y_list = []
    for i in range(0, params.num_cohorts):
        Y_j = estimateSetBits(reports[i], params)
        Y_list.append(Y_j)
    Y = np.array(Y_list)
    Y = Y.flatten()

    linreg = linear_model.LassoCV(n_alphas=10, cv=10)
    linreg.fit(X,Y)
    print("Coefficients: ", linreg.coef_)
    print("Alpha: ", linreg.alpha_)
    return X, Y, linreg, domains

def doLinearRegression(X, Y):
    X = sm.add_constant(X)
    ols = sm.OLS(Y,X)
    results = ols.fit()
    #Apply Bonferroni error correction
    bonferroni_alpha = 0.05/M
    #Remove the first p-value: it's for the added constant term, not a coefficient associated with a domain
    p_vals = results.pvalues[1:]
    
    return np.where(p_vals <= bonferroni_alpha)[0]
    
def testAllTheThings(numReportedDomains, numTotalDomains, numDuplicates):
    domainsToReport = getDomains(numReportedDomains)
    print("Domains to report:", domainsToReport)
    
    prob_fs = [0.0, 0.01, 0.1, 0.25, 0.5]
    coefs = {}
    for f in prob_fs:
        p = Params(prob_f=f)
        reports = generateReports(p, domainsToReport, numDuplicates)
        print("Reports:", len(reports[0]))
        coefs[f] = doLassoRegression(p, numTotalDomains, reports)
    return coefs

def readRapporReports(filename):
    reportFile = open(filename, 'r')
    reps = []
    for line in reportFile:
        reps.append(int(line.rstrip()))
    reportFile.close()
    return reps

def analyzeReports(filename, params, numDomains):
    reps = readRapporReports()
    lassX, lassY, lassoReg, domains = doLassoRegression(params, numDomains, reps)
    relevantDomains = np.where(lassoReg.coef_!=0)[0]
    linX = lassX[:,relevantDomains]
    #Keep track of the domains the LASSO selected
    linDoms = domains[relevantDomains]
    #Confirm that the domains LASSO selected are relevant using linear regression
    #This may prune out even more domains than LASSO did
    finalRelevantIdxs = doLinearRegression(linX, lassY)
    return linDoms[finalRelevantIdxs]

def main():
    rapporRepsFile = "rappor_reports.txt"
    numDomains = 10
    params = Params(prob_f=0.2)
    reportedDoms = analyzeReports(rapporRepsFile, params, numDomains)
    
    masterBlockedList = "master_blocked_list.txt"
    mbl = open(masterBlockedList, 'r+')
    alreadyBlocked = []
    for line in mbl:
        alreadyBlocked.append(line.rstrip())
    
    newBlocked = []
    for d in reportedDoms:
        if d not in alreadyBlocked:
            newBlocked.append(d)
    mbl.close()
    
    mbl = open(masterBlockedList, 'a')
    for d in newBlocked:
        mbl.write(d+"\r\n")
    





'''t0 = time.time()
coefs = testAllTheThings(5, 10, 10000, 91)
print(len(np.where(coefs != 0)[0]))
t1 = time.time()
print("Time taken: ", t1-t0)
#It sort of looks like alpha should scale based on number of features:
#if alpha=80 is right for 1,000, then alpha=800 is right for 10,000
'''













