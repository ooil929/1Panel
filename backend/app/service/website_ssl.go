package service

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"github.com/1Panel-dev/1Panel/backend/app/dto"
	"github.com/1Panel-dev/1Panel/backend/app/model"
	"github.com/1Panel-dev/1Panel/backend/utils/ssl"
	"strings"
)

type WebSiteSSLService struct {
}

func (w WebSiteSSLService) Page(search dto.WebsiteSSLSearch) (int64, []dto.WebsiteSSLDTO, error) {
	total, sslList, err := websiteSSLRepo.Page(search.Page, search.PageSize, commonRepo.WithOrderBy("created_at desc"))
	if err != nil {
		return 0, nil, err
	}
	var sslDTOs []dto.WebsiteSSLDTO
	for _, ssl := range sslList {
		sslDTOs = append(sslDTOs, dto.WebsiteSSLDTO{
			WebSiteSSL: ssl,
		})
	}
	return total, sslDTOs, err
}

func (w WebSiteSSLService) Search() ([]dto.WebsiteSSLDTO, error) {
	sslList, err := websiteSSLRepo.List()
	if err != nil {
		return nil, err
	}
	var sslDTOs []dto.WebsiteSSLDTO
	for _, ssl := range sslList {
		sslDTOs = append(sslDTOs, dto.WebsiteSSLDTO{
			WebSiteSSL: ssl,
		})
	}
	return sslDTOs, err
}

func (w WebSiteSSLService) Create(create dto.WebsiteSSLCreate) (dto.WebsiteSSLCreate, error) {

	var res dto.WebsiteSSLCreate
	acmeAccount, err := websiteAcmeRepo.GetFirst(commonRepo.WithByID(create.AcmeAccountID))
	if err != nil {
		return res, err
	}

	client, err := ssl.NewPrivateKeyClient(acmeAccount.Email, acmeAccount.PrivateKey)
	if err != nil {
		return res, err
	}

	switch create.Provider {
	case dto.DNSAccount:
		dnsAccount, err := websiteDnsRepo.GetFirst(commonRepo.WithByID(create.DnsAccountID))

		if err != nil {
			return res, err
		}

		if err := client.UseDns(ssl.DnsType(dnsAccount.Type), dnsAccount.Authorization); err != nil {
			return res, err
		}
	case dto.Http:
	case dto.DnsManual:

	}

	domains := []string{create.PrimaryDomain}
	otherDomainArray := strings.Split(create.OtherDomains, "\n")
	if create.OtherDomains != "" {
		domains = append(otherDomainArray, domains...)
	}
	resource, err := client.ObtainSSL(domains)
	if err != nil {
		return res, err
	}
	var websiteSSL model.WebSiteSSL
	websiteSSL.DnsAccountID = create.DnsAccountID
	websiteSSL.AcmeAccountID = acmeAccount.ID
	websiteSSL.Provider = string(create.Provider)
	websiteSSL.Domains = strings.Join(otherDomainArray, ",")
	websiteSSL.PrimaryDomain = create.PrimaryDomain
	websiteSSL.PrivateKey = string(resource.PrivateKey)
	websiteSSL.Pem = string(resource.Certificate)
	websiteSSL.CertURL = resource.CertURL
	certBlock, _ := pem.Decode(resource.Certificate)
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return dto.WebsiteSSLCreate{}, err
	}
	websiteSSL.ExpireDate = cert.NotAfter
	websiteSSL.StartDate = cert.NotBefore
	websiteSSL.Type = cert.Issuer.CommonName
	websiteSSL.Organization = cert.Issuer.Organization[0]

	//if err := createPemFile(websiteSSL); err != nil {
	//	return dto.WebsiteSSLCreate{}, err
	//}

	if err := websiteSSLRepo.Create(context.TODO(), &websiteSSL); err != nil {
		return res, err
	}

	return create, nil
}

func (w WebSiteSSLService) Renew(sslId uint) error {

	websiteSSL, err := websiteSSLRepo.GetFirst(commonRepo.WithByID(sslId))
	if err != nil {
		return err
	}
	acmeAccount, err := websiteAcmeRepo.GetFirst(commonRepo.WithByID(websiteSSL.AcmeAccountID))
	if err != nil {
		return err
	}

	client, err := ssl.NewPrivateKeyClient(acmeAccount.Email, acmeAccount.PrivateKey)
	if err != nil {
		return err
	}
	switch websiteSSL.Provider {
	case dto.DNSAccount:
		dnsAccount, err := websiteDnsRepo.GetFirst(commonRepo.WithByID(websiteSSL.DnsAccountID))
		if err != nil {
			return err
		}
		if err := client.UseDns(ssl.DnsType(dnsAccount.Type), dnsAccount.Authorization); err != nil {
			return err
		}
	case dto.Http:
	case dto.DnsManual:

	}

	resource, err := client.RenewSSL(websiteSSL.CertURL)
	if err != nil {
		return err
	}
	websiteSSL.PrivateKey = string(resource.PrivateKey)
	websiteSSL.Pem = string(resource.Certificate)
	websiteSSL.CertURL = resource.CertURL
	certBlock, _ := pem.Decode(resource.Certificate)
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return err
	}
	websiteSSL.ExpireDate = cert.NotAfter
	websiteSSL.StartDate = cert.NotBefore
	websiteSSL.Type = cert.Issuer.CommonName
	websiteSSL.Organization = cert.Issuer.Organization[0]

	return websiteSSLRepo.Save(websiteSSL)
}

func (w WebSiteSSLService) GetDNSResolve(req dto.WebsiteDNSReq) (dto.WebsiteDNSRes, error) {
	acmeAccount, err := websiteAcmeRepo.GetFirst(commonRepo.WithByID(req.AcmeAccountID))
	if err != nil {
		return dto.WebsiteDNSRes{}, err
	}

	client, err := ssl.NewPrivateKeyClient(acmeAccount.Email, acmeAccount.PrivateKey)
	if err != nil {
		return dto.WebsiteDNSRes{}, err
	}
	re, err := client.UseManualDns(req.Domains)
	if err != nil {
		return dto.WebsiteDNSRes{}, err
	}
	var res dto.WebsiteDNSRes
	res.Key = re.Key
	res.Value = re.Value
	res.Type = "TXT"
	return res, nil
}

func (w WebSiteSSLService) GetWebsiteSSL(websiteId uint) (dto.WebsiteSSLDTO, error) {
	var res dto.WebsiteSSLDTO
	website, err := websiteRepo.GetFirst(commonRepo.WithByID(websiteId))
	if err != nil {
		return res, err
	}
	websiteSSL, err := websiteSSLRepo.GetFirst(commonRepo.WithByID(website.WebSiteSSLID))
	if err != nil {
		return res, err
	}
	res.WebSiteSSL = websiteSSL
	return res, nil
}

func (w WebSiteSSLService) Delete(id uint) error {
	return websiteSSLRepo.DeleteBy(commonRepo.WithByID(id))
}